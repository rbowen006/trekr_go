package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rbowen/trekr_go/internal/httpapi/middleware"
)

// aiRateLimitWindow and aiRateLimit cap paid Claude calls at 10 per hour, per
// user, per feature (ADR-0010) — one bucket for descriptions, one for replies.
const (
	aiRateLimitWindow = time.Hour
	aiRateLimit       = 10
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

		r.Route("/api/v1", func(r chi.Router) {
			// Public reads (Rails skips authenticate_user! for these).
			r.Get("/listings", app.listingsIndex)
			r.Get("/listings/{id}", app.listingsShow)

			// Public semantic search. OptionalAuth attributes the caller to the
			// nl_search ai_requests row when a token is present, without
			// requiring one (mirrors skip_before_action :authenticate_user!).
			r.With(middleware.OptionalAuth(app.Config.SecretKeyBase, app.DB)).
				Post("/listings/search", app.listingsSearch)

			// Everything else under /api/v1 requires authentication. The
			// catch-all keeps the auth boundary deterministic so unauthenticated
			// requests to unmapped routes get a JSend 401 rather than a bare 404.
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth(app.Config.SecretKeyBase, app.DB))

				r.Get("/listings/mine", app.listingsMine)
				r.Post("/listings", app.listingsCreate)
				r.Put("/listings/{id}", app.listingsUpdate)
				r.Patch("/listings/{id}", app.listingsUpdate)
				r.Delete("/listings/{id}", app.listingsDestroy)

				r.Post("/listings/{listing_id}/images", app.imagesCreate)
				r.Delete("/listings/{listing_id}/images/{id}", app.imagesDestroy)

				r.Post("/listings/{listing_id}/bookings", app.bookingsCreate)
				r.Get("/bookings", app.bookingsIndex)
				r.Get("/bookings/{id}", app.bookingsShow)
				r.Patch("/bookings/{id}/confirm", app.bookingsConfirm)
				r.Patch("/bookings/{id}/reject", app.bookingsReject)

				r.Post("/listings/{listing_id}/chats", app.chatsCreate)
				r.Get("/chats", app.chatsIndex)
				r.Get("/chats/{id}", app.chatsShow)
				r.Get("/chats/{chat_id}/messages", app.messagesIndex)
				r.Post("/chats/{chat_id}/messages", app.messagesCreate)

				// AI endpoints (PR #15). Each carries its own per-user, per-feature
				// rate limit that increments on admission — so a throttled request
				// never reaches Claude. The limiter falls back to in-memory when
				// none is wired (tests/CI need no Redis).
				limiter := app.Limiter
				if limiter == nil {
					limiter = middleware.NewMemoryRateLimiter()
				}
				r.With(middleware.RateLimit(limiter, "description_generator", aiRateLimit, aiRateLimitWindow)).
					Post("/listings/generate_description", app.generateDescription)
				r.With(middleware.RateLimit(limiter, "chat_reply", aiRateLimit, aiRateLimitWindow)).
					Post("/chats/{chat_id}/suggest_reply", app.suggestReply)

				r.HandleFunc("/*", func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "not found", http.StatusNotFound)
				})
			})
		})
	}
	return r
}
