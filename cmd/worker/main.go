// Command worker runs the asynq background worker that processes embedding
// jobs (ADR-0011). It mirrors the Rails ActiveJob worker: enqueue happens in
// the API process, execution happens here.
package main

import (
	"log"

	"github.com/hibiken/asynq"
	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/db"
	"github.com/rbowen/trekr_go/internal/jobs"
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

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}

	embedder := ai.NewEmbedder(database, cfg.OllamaURL)
	srv := asynq.NewServer(redisOpt, asynq.Config{})

	log.Printf("trekr_go worker starting")
	if err := srv.Run(jobs.NewServeMux(database, embedder)); err != nil {
		log.Fatalf("worker: %v", err)
	}
}
