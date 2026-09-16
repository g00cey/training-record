// Package config loads backend configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config is the fully-resolved backend configuration.
type Config struct {
	Port                string // PORT (default 8080)
	DBPath              string // DB_PATH (default ./training.db)
	APIKey              string // API_KEY (required)
	BootstrapDB         string // BOOTSTRAP_DB_PATH (optional)
	TZ                  string // TZ (optional; informational + timezone override)
	HermesURL           string // HERMES_API_URL (optional; enables POST /api/spin-extract)
	HermesKey           string // HERMES_API_KEY (optional; bearer key for the Hermes extraction API)
	LLMEvalURL          string // LLM_EVAL_API_URL (optional; enables `server -evaluate-training`)
	LLMEvalKey          string // LLM_EVAL_API_KEY (optional; bearer key for the LLM evaluation API)
	LLMEvalHistoryWeeks int    // LLM_EVAL_HISTORY_WEEKS (default 8)
}

// Load reads configuration from the environment. APIKey is required.
func Load() (*Config, error) {
	c := &Config{
		Port:                envOr("PORT", "8080"),
		DBPath:              envOr("DB_PATH", "./training.db"),
		APIKey:              os.Getenv("API_KEY"),
		BootstrapDB:         os.Getenv("BOOTSTRAP_DB_PATH"),
		TZ:                  os.Getenv("TZ"),
		HermesURL:           os.Getenv("HERMES_API_URL"),
		HermesKey:           os.Getenv("HERMES_API_KEY"),
		LLMEvalURL:          os.Getenv("LLM_EVAL_API_URL"),
		LLMEvalKey:          os.Getenv("LLM_EVAL_API_KEY"),
		LLMEvalHistoryWeeks: envOrInt("LLM_EVAL_HISTORY_WEEKS", 8),
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

func envOrInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
