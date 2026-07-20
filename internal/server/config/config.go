package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds server runtime config, sourced from environment.
type Config struct {
	Driver      string // "sqlite" (default) or "pgx"
	DatabaseURL string
	ListenAddr  string
}

// Load reads config from env. DEVSYNC_DATABASE_URL is OPTIONAL: a postgres://
// URL selects Postgres; anything else (including empty) uses a local SQLite file
// so the server runs single-binary with no external database.
func Load() (*Config, error) {
	raw := os.Getenv("DEVSYNC_DATABASE_URL")

	var driver, dsn string
	if strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://") {
		driver, dsn = "pgx", raw
	} else {
		driver = "sqlite"
		path := raw
		if path == "" {
			dir, err := os.UserConfigDir()
			if err != nil {
				return nil, fmt.Errorf("resolve config dir: %w", err)
			}
			dir = filepath.Join(dir, "devsync")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("create data dir: %w", err)
			}
			path = filepath.Join(dir, "devsync.db")
		}
		// Pragmas: FK enforcement on, wait on locks, WAL for concurrent reads.
		dsn = "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}

	addr := os.Getenv("DEVSYNC_LISTEN_ADDR")
	if addr == "" {
		if p := os.Getenv("PORT"); p != "" {
			addr = ":" + p
		} else {
			addr = ":8080"
		}
	}
	return &Config{Driver: driver, DatabaseURL: dsn, ListenAddr: addr}, nil
}
