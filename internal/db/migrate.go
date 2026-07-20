package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib" // "pgx" driver
	_ "modernc.org/sqlite"             // "sqlite" driver
)

//go:embed migrations/postgres/*.sql migrations/sqlite/*.sql
var migrationsFS embed.FS

// Migrate runs all up migrations against the given DSN. Idempotent. driver is
// "pgx" (Postgres) or "sqlite"; each backend has its own migrations subdir and
// goose_db_version table, so version numbers never collide.
func Migrate(ctx context.Context, driver, dsn string) error {
	dialect, dir := "postgres", "migrations/postgres"
	if driver == "sqlite" {
		dialect, dir = "sqlite3", "migrations/sqlite"
	}

	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
