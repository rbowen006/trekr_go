package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port           string `env:"PORT" envDefault:"3000"`
	AllowedOrigins string `env:"ALLOWED_ORIGINS" envDefault:"http://localhost:5173"`
	DatabaseURL    string `env:"DATABASE_URL" envDefault:"postgres://postgres:password@localhost:5433/rv_marketplace_test"`
	// SecretKeyBase signs JWTs; must equal Rails' SECRET_KEY_BASE for cross-backend tokens.
	SecretKeyBase string `env:"SECRET_KEY_BASE"`
	// StorageRoot is the Active Storage disk root (points at rv_marketplace/storage).
	StorageRoot string `env:"STORAGE_ROOT"`
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
