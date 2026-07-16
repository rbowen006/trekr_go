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
	// OllamaURL is the base URL of the local Ollama server used for embeddings
	// (ADR-0011). Host-facing default; docker sets http://ollama:11434.
	OllamaURL string `env:"OLLAMA_URL" envDefault:"http://localhost:11434"`
	// RedisURL backs the asynq job queue and the AI rate limiter. Host-facing
	// default; docker sets redis://redis:6379/0.
	RedisURL string `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`
	// AnthropicAPIKey authenticates Claude Messages API calls (the AI slice,
	// PR #15). Empty in tests, which stub Anthropic with an httptest server.
	AnthropicAPIKey string `env:"ANTHROPIC_API_KEY"`
	// AnthropicBaseURL is the Anthropic API base URL. Overridable so tests can
	// point the SDK at a local httptest fake (mirrors the WebMock stub of
	// https://api.anthropic.com/v1/messages in the Rails specs).
	AnthropicBaseURL string `env:"ANTHROPIC_BASE_URL" envDefault:"https://api.anthropic.com"`
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
