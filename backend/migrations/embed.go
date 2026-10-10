// Package migrations — SQL-схема БД (перенесена из backend/prisma/migrations без изменений).
package migrations

import _ "embed"

//go:embed 000001_init.up.sql
var Init string

//go:embed 000002_remove_applications.up.sql
var RemoveApplications string

//go:embed 000003_add_gamification.up.sql
var AddGamification string

// Migration — именованная миграция (применяется по порядку, один раз).
type Migration struct {
	Name string
	SQL  string
}

// All — порядок применения (соответствует prisma migrate deploy).
var All = []Migration{
	{Name: "0_init", SQL: Init},
	{Name: "20261008000000_remove_applications", SQL: RemoveApplications},
	{Name: "20261008214509_add_gamification", SQL: AddGamification},
}
