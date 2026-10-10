// Package health — GET /api/health (диагностика DB/Redis/embedding).
package health

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/store"
)

// EmbeddingInfo — состояние embedding-провайдера (реализует matching.EmbeddingService).
type EmbeddingInfo interface {
	CurrentProvider() string
	CurrentModel() string
	Dim() int
}

type CacheInfo interface {
	IsAvailable() bool
}

type Service struct {
	db        *store.DB
	cache     CacheInfo
	embedding EmbeddingInfo
	startedAt time.Time
}

func NewService(db *store.DB, cache CacheInfo, embedding EmbeddingInfo) *Service {
	return &Service{db: db, cache: cache, embedding: embedding, startedAt: time.Now()}
}

func (s *Service) Check(ctx context.Context) map[string]any {
	database := "down"
	if err := s.db.Ping(ctx); err == nil {
		database = "up"
	}
	redisState := "down (cache disabled)"
	if s.cache.IsAvailable() {
		redisState = "up"
	}
	embeddingInfo := map[string]any{
		"provider": s.embedding.CurrentProvider(),
		"model":    s.embedding.CurrentModel(),
		"dim":      s.embedding.Dim(),
	}
	status := "degraded"
	if database == "up" {
		status = "ok"
	}
	uptime := int(time.Since(s.startedAt).Seconds() + 0.5)
	// Формат ответа — по docs 04 §11 (плюс services.* — вложенный вид из исходников).
	return map[string]any{
		"status":        status,
		"timestamp":     time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"uptimeSeconds": uptime,
		"database":      database,
		"redis":         redisState,
		"embedding":     embeddingInfo,
		"services": map[string]any{
			"database":  database,
			"redis":     redisState,
			"embedding": embeddingInfo,
		},
	}
}

// Handler — HTTP-слой.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — GET /api/health (@Public).
func (h *Handler) Register(r *common.Router) {
	r.GET("/health", common.RouteOpts{Public: true}, h.check)
}

func (h *Handler) check(c *gin.Context) error {
	common.JSON(c, 200, h.service.Check(c.Request.Context()))
	return nil
}
