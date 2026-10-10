package store

import (
	"context"
	"fmt"

	"fsp-backend/migrations"
)

// Migrate применяет SQL-миграции (аналог `npx prisma migrate deploy` при старте контейнера).
func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return err
	}
	// Перенос отметок Prisma: если БД уже накатена `prisma migrate deploy`,
	// имена миграций совпадают — не переопляем созданную схему.
	var hasPrisma bool
	if err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = '_prisma_migrations')`,
	).Scan(&hasPrisma); err != nil {
		return err
	}
	if hasPrisma {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO schema_migrations (name)
			SELECT migration_name FROM _prisma_migrations
			WHERE finished_at IS NOT NULL
			ON CONFLICT DO NOTHING`); err != nil {
			return err
		}
	}
	for _, m := range migrations.All {
		var exists bool
		if err := db.Pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, m.Name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", m.Name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (name) VALUES ($1)`, m.Name); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
