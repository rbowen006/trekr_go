package httpapi

import (
	"github.com/rbowen/trekr_go/internal/config"
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
}
