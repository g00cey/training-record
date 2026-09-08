// Package config loads backend configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config is the fully-resolved backend configuration.
type Config struct {
	Port        string // PORT (default 8080)
	DBPath      string // DB_PATH (default ./training.db)
	APIKey      string // API_KEY (required)
	BootstrapDB string // BOOTSTRAP_DB_PATH (optional)
	TZ          string // TZ (optional; informational + timezone override)
}

// Load reads configuration from the environment. APIKey is required.
func Load() (*Config, error) {
	c := &Config{
		Port:        envOr("PORT", "8080"),
		DBPath:      envOr("DB_PATH", "./training.db"),
		APIKey:      os.Getenv("API_KEY"),
		BootstrapDB: os.Getenv("BOOTSTRAP_DB_PATH"),
		TZ:          os.Getenv("TZ"),
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("API_KEY is required")
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
