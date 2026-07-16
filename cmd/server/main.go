package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/db"
	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/jobs"
	"github.com/rbowen/trekr_go/internal/ratelimit"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	queue, err := jobs.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("queue: %v", err)
	}
	defer func() { _ = queue.Close() }()

	limiter, err := ratelimit.NewRedisLimiter(cfg.RedisURL)
	if err != nil {
		log.Fatalf("rate limiter: %v", err)
	}
	defer func() { _ = limiter.Close() }()

	app := &httpapi.App{
		Config:     cfg,
		DB:         database,
		EmbedQueue: queue,
		Claude:     ai.NewClaude(database, cfg.AnthropicAPIKey, cfg.AnthropicBaseURL),
		Limiter:    limiter,
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("trekr_go listening on %s", addr)

	if err := http.ListenAndServe(addr, httpapi.NewRouter(app)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
