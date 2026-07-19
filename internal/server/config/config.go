package config

import (
	"fmt"
	"os"
)

// Config holds server runtime config, sourced from environment.
type Config struct {
	DatabaseURL string
	ListenAddr  string
}

// Load reads config from env. DEVSYNC_DATABASE_URL is required.
func Load() (*Config, error) {
	dsn := os.Getenv("DEVSYNC_DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DEVSYNC_DATABASE_URL is required")
	}
	addr := os.Getenv("DEVSYNC_LISTEN_ADDR")
	if addr == "" {
		if p := os.Getenv("PORT"); p != "" {
			addr = ":" + p
		} else {
			addr = ":8080"
		}
	}
	return &Config{DatabaseURL: dsn, ListenAddr: addr}, nil
}
