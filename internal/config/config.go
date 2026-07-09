package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port           string `env:"PORT" envDefault:"3000"`
	AllowedOrigins string `env:"ALLOWED_ORIGINS" envDefault:"http://localhost:5173"`
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
