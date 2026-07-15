package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rbowen/trekr_go/internal/auth"
	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

type contextKey string

const currentUserKey contextKey = "currentUser"

// unauthenticatedMessage mirrors Devise's failure.unauthenticated i18n message.
const unauthenticatedMessage = "You need to sign in or sign up before continuing."

// RequireAuth authenticates requests via a devise-jwt Bearer token. On success
// it loads the user and stores it in the request context; otherwise it responds
// with Rails' JSend 401 fail shape. Accepts tokens issued by either backend.
func RequireAuth(secret string, db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := resolveUser(secret, db, r)
			if !ok {
				writeJSendFail(w, http.StatusUnauthorized, unauthenticatedMessage)
				return
			}
			ctx := context.WithValue(r.Context(), currentUserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth loads the user into the request context when a valid Bearer
// token is present, but never rejects the request. Public endpoints that still
// attribute the caller when known (e.g. semantic search logging ai_requests)
// use it, mirroring Warden running under Rails' skip_before_action.
func OptionalAuth(secret string, db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user, ok := resolveUser(secret, db, r); ok {
				r = r.WithContext(context.WithValue(r.Context(), currentUserKey, user))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveUser parses the request's Bearer token and loads the corresponding
// user, reporting ok=false when the token is missing, invalid, or the user no
// longer exists. Shared by RequireAuth (which rejects on !ok) and OptionalAuth
// (which simply proceeds).
func resolveUser(secret string, db *gorm.DB, r *http.Request) (*models.User, bool) {
	token := bearerToken(r)
	if token == "" {
		return nil, false
	}
	userID, err := auth.ParseToken(secret, token)
	if err != nil {
		return nil, false
	}
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		return nil, false
	}
	return &user, true
}

// CurrentUser returns the authenticated user stored by RequireAuth, if any.
func CurrentUser(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(currentUserKey).(*models.User)
	return user, ok
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
