package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultServerURL is set at build time via ldflags for a zero-config client.
// Example: go build -ldflags "-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=https://devworkspace.onrender.com"
var DefaultServerURL string

// Config is the client's persisted state in ~/.devsync/config.json.
// Private keys live in separate files, never here.
type Config struct {
	ServerURL string `json:"server_url,omitempty"`
	Username  string `json:"username,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
}

// Dir returns ~/.devsync, creating it if missing.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ".devsync")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads config.json. Returns empty Config if it does not exist yet.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{ServerURL: DefaultServerURL}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.ServerURL == "" {
		c.ServerURL = DefaultServerURL
	}
	return &c, nil
}

// Save writes config.json atomically with 0600 perms.
func (c *Config) Save() error {
	p, err := path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Rename(tmp, p)
}
