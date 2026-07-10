package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rbowen/trekr_go/internal/httpapi/middleware"
)

// NewRouter returns the application HTTP router with all routes registered.
func NewRouter(app *App) http.Handler {
	r := chi.NewRouter()

	origins := strings.Split(app.Config.AllowedOrigins, ",")
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
	r.Delete("/users/sign_out", app.signOut)
	if app.DB != nil {
		r.Post("/users", app.registerUser)
		r.Post("/users/sign_in", app.signIn)
		r.Post("/users/password", app.createPasswordReset)
		r.Put("/users/password", app.updatePasswordReset)
		r.Patch("/users/password", app.updatePasswordReset)

		// Active Storage read path (public, like Rails): permanent signed blob
		// URLs redirect to expiring disk-service URLs that serve file bytes.
		r.Get("/rails/active_storage/blobs/redirect/*", app.blobRedirect)
		r.Get("/rails/active_storage/disk/*", app.diskShow)

		// All /api/v1 endpoints require authentication. Concrete routes land
		// in later PRs; the catch-all keeps the auth boundary deterministic so
		// unauthenticated requests get a JSend 401 rather than a bare 404.
		r.Route("/api/v1", func(r chi.Router) {
			r.Use(middleware.RequireAuth(app.Config.SecretKeyBase, app.DB))
			r.HandleFunc("/*", func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			})
		})
	}
	return r
}
