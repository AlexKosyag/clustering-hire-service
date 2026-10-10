// cmd/seed — наполнение демо-данными (порт prisma/seed.ts).
// Запуск: go run ./cmd/seed (нужен DATABASE_URL; перед запуском применяются миграции).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"fsp-backend/internal/config"
	"fsp-backend/internal/seed"
	"fsp-backend/internal/store"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fail(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		fail(fmt.Errorf("миграции: %w", err))
	}

	if err := seed.Run(ctx, db, os.Stdout); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Seed failed:", err)
	os.Exit(1)
}
