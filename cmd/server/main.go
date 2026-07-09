package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/db"
	"github.com/rbowen/trekr_go/internal/httpapi"
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

	app := &httpapi.App{
		Config: cfg,
		DB:     database,
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("trekr_go listening on %s", addr)

	if err := http.ListenAndServe(addr, httpapi.NewRouter(app)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
