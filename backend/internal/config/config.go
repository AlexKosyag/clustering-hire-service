// Package config — типизированная конфигурация (аналог src/config/configuration.ts).
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        int
	DatabaseURL string
	RedisURL    string

	JWTSecret        string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration // 15m
	RefreshTokenTTL  time.Duration // 7d

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	// MinioPublicEndpoint — базовый URL публичных ссылок (читается напрямую из env).
	MinioPublicEndpoint string

	EmbeddingProvider string // auto | tfidf | transformers

	MatchingCacheTTLSeconds int

	TestingQuestionsPerTest int
	TestingCooldownDays     int

	// Переменные, читаемые только из process.env (см. docs 01 §7).
	FrontendURL           string
	MLServiceURL          string
	MLServiceTimeout      time.Duration
	TestingTasksPerTest   int
	TestingV2CooldownDays int
	OpenAIAPIKey          string
	OpenAIAPIURL          string
	OpenAIModel           string
	SandboxTimeout        time.Duration
	SandboxMemoryMB       int
	SandboxDocker         bool
	NodeEnv               string
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func Load() *Config {
	return &Config{
		Port:        envInt("PORT", 3000),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    envStr("REDIS_URL", "redis://redis:6379"),

		JWTSecret:        envStr("JWT_SECRET", "dev-secret"),
		JWTRefreshSecret: envStr("JWT_REFRESH_SECRET", "dev-refresh-secret"),
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  7 * 24 * time.Hour,

		MinioEndpoint:       envStr("MINIO_ENDPOINT", "http://minio:9000"),
		MinioAccessKey:      envStr("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:      envStr("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:         envStr("MINIO_BUCKET", "fsp-platform"),
		MinioPublicEndpoint: envStr("MINIO_PUBLIC_ENDPOINT", "http://localhost:9000"),

		EmbeddingProvider: envStr("EMBEDDING_PROVIDER", "auto"),

		MatchingCacheTTLSeconds: 900,

		TestingQuestionsPerTest: 20,
		TestingCooldownDays:     90,

		FrontendURL:           envStr("FRONTEND_URL", "http://localhost:8080"),
		MLServiceURL:          envStr("ML_SERVICE_URL", "http://ml-service:8000"),
		MLServiceTimeout:      time.Duration(envInt("ML_SERVICE_TIMEOUT", 5000)) * time.Millisecond,
		TestingTasksPerTest:   envInt("TESTING_TASKS_PER_TEST", 15),
		TestingV2CooldownDays: envInt("TESTING_COOLDOWN_DAYS", 90),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIAPIURL:          envStr("OPENAI_API_URL", "https://api.openai.com/v1/chat/completions"),
		OpenAIModel:           envStr("OPENAI_MODEL", "gpt-4o-mini"),
		SandboxTimeout:        time.Duration(envInt("SANDBOX_TIMEOUT_MS", 10000)) * time.Millisecond,
		SandboxMemoryMB:       envInt("SANDBOX_MEMORY_MB", 256),
		SandboxDocker:         envBool("SANDBOX_DOCKER", false),
		NodeEnv:               envStr("NODE_ENV", "development"),
	}
}

// CORSOrigains — список разрешённых origin (main.ts).
func (c *Config) CORSOrigains() []string {
	return []string{c.FrontendURL, "http://localhost:8080", "http://localhost:3000", "http://127.0.0.1:8080"}
}
