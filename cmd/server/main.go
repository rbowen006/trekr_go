package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("trekr_go listening on %s", addr)

	if err := http.ListenAndServe(addr, httpapi.NewRouter(cfg)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
