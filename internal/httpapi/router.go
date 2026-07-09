package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/httpapi/middleware"
)

// NewRouter returns the application HTTP router with all routes registered.
func NewRouter(cfg config.Config) http.Handler {
	r := chi.NewRouter()

	origins := strings.Split(cfg.AllowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"Authorization"},
		AllowCredentials: false,
	}))
	r.Use(middleware.MalformedJSON)

	r.Get("/up", healthHandler)
	return r
}
