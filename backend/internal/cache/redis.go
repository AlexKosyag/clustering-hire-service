// Package cache — Redis-клиент с полной деградацией (аналог common/redis/redis.service.ts).
// Назначение — только кэширование результатов matching (ключи matching:cache:*).
package cache

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client  *redis.Client
	mu      sync.Mutex
	ready   bool
	blocked time.Time // circuit-breaker: blockedUntil = now + 30s при ошибке
}

// New подключается к Redis по URL и отслеживает доступность в фоне.
func New(url string) *Redis {
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("[Redis] некорректный URL: %s", err.Error())
		return &Redis{}
	}
	opt.MaxRetries = 1
	opt.DialTimeout = 2 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second
	r := &Redis{client: redis.NewClient(opt)}
	go r.watch()
	return r
}

func (r *Redis) watch() {
	for {
		if r.client == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := r.client.Ping(ctx).Err()
		cancel()
		r.mu.Lock()
		if err == nil {
			if !r.ready {
				log.Println("[Redis] Redis подключён")
			}
			r.ready = true
			r.blocked = time.Time{}
		} else {
			if r.ready {
				log.Printf("[Redis] Redis недоступен: %s — продолжаем без кэша", err.Error())
			}
			r.ready = false
			r.blocked = time.Now().Add(30 * time.Second)
		}
		r.mu.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func (r *Redis) Close() {
	if r.client != nil {
		_ = r.client.Close()
	}
}

// IsAvailable — клиент жив и не заблокирован circuit-breaker-ом.
func (r *Redis) IsAvailable() bool {
	if r.client == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready && time.Now().After(r.blocked)
}

func (r *Redis) markDown() {
	r.mu.Lock()
	r.ready = false
	r.blocked = time.Now().Add(30 * time.Second)
	r.mu.Unlock()
}

// Get возвращает значение или nil (молча при недоступности).
func (r *Redis) Get(ctx context.Context, key string) *string {
	if !r.IsAvailable() {
		return nil
	}
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			r.markDown()
		}
		return nil
	}
	return &val
}

// Set пишет значение с опциональным TTL (SET ... EX).
func (r *Redis) Set(ctx context.Context, key, value string, ttlSeconds ...int) {
	if !r.IsAvailable() {
		return
	}
	var err error
	if len(ttlSeconds) > 0 && ttlSeconds[0] > 0 {
		err = r.client.Set(ctx, key, value, time.Duration(ttlSeconds[0])*time.Second).Err()
	} else {
		err = r.client.Set(ctx, key, value, 0).Err()
	}
	if err != nil {
		r.markDown()
	}
}

// DelByPattern — SCAN + DEL по шаблону (инвалидация кэша matching).
func (r *Redis) DelByPattern(ctx context.Context, pattern string) {
	if !r.IsAvailable() {
		return
	}
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			r.markDown()
			return
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				r.markDown()
				return
			}
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}
