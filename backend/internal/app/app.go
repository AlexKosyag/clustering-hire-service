// Package app — сборка всего приложения (все сервисы, роутер, Swagger).
// Используется cmd/server (прод) и test/e2e (in-process на httptest).
// Аналог AppModule из src/app.module.ts + src/main.ts.
package app

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/cache"
	"fsp-backend/internal/common"
	"fsp-backend/internal/config"
	"fsp-backend/internal/modules/auth"
	"fsp-backend/internal/modules/candidates"
	"fsp-backend/internal/modules/employers"
	"fsp-backend/internal/modules/fsp"
	"fsp-backend/internal/modules/gamification"
	"fsp-backend/internal/modules/health"
	"fsp-backend/internal/modules/invitations"
	"fsp-backend/internal/modules/matching"
	"fsp-backend/internal/modules/notifications"
	"fsp-backend/internal/modules/storage"
	testingmod "fsp-backend/internal/modules/testing"
	"fsp-backend/internal/modules/vacancies"
	"fsp-backend/internal/store"
)

// App — собранное приложение.
type App struct {
	Engine  *gin.Engine
	DB      *store.DB
	Redis   *cache.Redis
	Cfg     *config.Config
	Matcher *matching.Service
}

// New — подключение к БД, миграции, все сервисы и маршруты (аналог AppModule).
func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}

	redis := cache.New(cfg.RedisURL)

	if cfg.NodeEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(
		common.RecoveryMiddleware(),
		common.LoggingMiddleware(),
		common.CORSMiddleware(cfg.CORSOrigains()),
	)

	// Guard по умолчанию закрывает всё; маршруты помечаются Public/Roles явно.
	router := &common.Router{
		Engine:       engine,
		Authenticate: auth.NewAuthGuard(db, cfg.JWTSecret),
	}

	// ===== Сервисы =====
	vectors := store.NewVectorRepository(db)
	storageSvc := storage.New(cfg)
	gamSvc := gamification.NewService(db)

	matcher := matching.NewService(db, vectors, cfg, redis)

	authSvc := auth.NewService(db, cfg)
	candidatesSvc := candidates.NewService(db, matcher, storageSvc)
	vacanciesSvc := vacancies.NewService(db)
	employersSvc := employers.NewService(db, matcher, storageSvc)
	invitationsSvc := invitations.NewService(db, matcher)
	fspSvc := fsp.NewService(db, matcher)
	notificationsSvc := notifications.NewService(db)

	tStore := testingmod.NewPGStore(db)
	tCfg := testingmod.TestingConfig{
		QuestionsPerTest: cfg.TestingQuestionsPerTest,
		CooldownDays:     cfg.TestingCooldownDays,
		TasksPerTest:     cfg.TestingTasksPerTest,
		V2CooldownDays:   cfg.TestingV2CooldownDays,
	}
	tV1 := testingmod.NewService(tStore, tCfg, matcher)
	tV2 := testingmod.NewServiceV2(tStore, tCfg, matcher, gamSvc)

	// ===== Маршруты (порядок как в AppModule) =====
	health.NewHandler(health.NewService(db, redis, matcher)).Register(router)
	auth.NewHandler(authSvc).Register(router)
	candidates.NewHandler(candidatesSvc).Register(router)
	vacancies.NewHandler(vacanciesSvc).Register(router)
	employers.NewHandler(employersSvc).Register(router)
	invitations.NewHandler(invitationsSvc).Register(router)
	fsp.NewHandler(fspSvc).Register(router)
	gamification.NewHandler(gamSvc).Register(router)
	notifications.NewHandler(notificationsSvc).Register(router)
	testingmod.NewHandler(tV1, tV2).Register(router)
	matcher.RegisterRoutes(router)

	registerSwagger(engine)

	return &App{Engine: engine, DB: db, Redis: redis, Cfg: cfg, Matcher: matcher}, nil
}

// Close — освобождение ресурсов.
func (a *App) Close() {
	if a.Redis != nil {
		a.Redis.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
	log.Println("[App] ресурсы освобождены")
}
