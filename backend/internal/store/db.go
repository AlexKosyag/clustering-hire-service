// Package store — доступ к PostgreSQL (аналог PrismaService): пул pgx и raw SQL для pgvector.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB — обёртка над пулом соединений.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect открывает пул по DATABASE_URL.
func Connect(ctx context.Context, dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is empty")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() { db.Pool.Close() }

// Ping — проверка доступности БД (для /api/health).
func (db *DB) Ping(ctx context.Context) error { return db.Pool.Ping(ctx) }
