package httpapi

import (
	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/config"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/internal/services"
	"gorm.io/gorm"
)

// App holds shared dependencies for HTTP handlers.
type App struct {
	Config config.Config
	DB     *gorm.DB
	// EmbedQueue enqueues listing-embed jobs on listing create/update
	// (ADR-0011). Nil in tests that don't exercise embeddings.
	EmbedQueue services.ListingEmbedQueue
	// Claude runs the Anthropic-backed AI endpoints (PR #15). Tests set it with
	// a BaseURL pointing at an httptest fake; nil elsewhere.
	Claude *ai.Claude
	// Limiter backs the AI rate limit. When nil, NewRouter falls back to an
	// in-memory limiter so the suite needs no Redis.
	Limiter mw.RateLimiter
}
