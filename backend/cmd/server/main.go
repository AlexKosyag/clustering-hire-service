// cmd/server — точка входа API (аналог src/main.ts): сборка приложения,
// миграции при старте, миграции, CORS, guard-ы, Swagger.
//
// @title FSP Platform API
// @version 1.0
// @description API цифровой платформы ФСП (Go-порт NestJS-бэкенда, контракт docs/04).
// @BasePath /
// @securityDefinitions.apikey bearer
// @in header
// @name Authorization
// @description JWT access token (scheme: bearer, format: JWT)
package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"fsp-backend/internal/app"
	"fsp-backend/internal/config"
)

func main() {
	cfg := config.Load()

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("[Bootstrap] %s", err.Error())
	}
	defer application.Close()

	addr := ":" + strconv.Itoa(cfg.Port)
	log.Printf("[Bootstrap] API: http://localhost:%d/api", cfg.Port)
	log.Printf("[Bootstrap] Swagger: http://localhost:%d/api/docs", cfg.Port)
	if err := application.Engine.Run(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[Bootstrap] %s", err.Error())
	}
}
