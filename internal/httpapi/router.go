package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter returns the application HTTP router with all routes registered.
func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/up", healthHandler)
	return r
}
