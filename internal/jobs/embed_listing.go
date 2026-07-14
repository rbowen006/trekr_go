package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/pgvector/pgvector-go"
	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

// EmbedListing regenerates a listing's semantic-search embedding (ADR-0011).
// Idempotent: it re-embeds only when the composed document is missing or its
// content_hash changed, so edits outside the document (price, lat/lng) are
// no-ops. Mirrors GenerateListingEmbeddingJob#perform. A missing listing is a
// no-op (no Ollama call, no ai_requests row).
func EmbedListing(ctx context.Context, db *gorm.DB, embedder *ai.Embedder, listingID int64) error {
	// Limit(1).Find (not First) so a missing listing is a clean no-op without
	// GORM logging ErrRecordNotFound — the job runs for every write, including
	// for listings deleted before it executes.
	var listing models.RvListing
	if res := db.Limit(1).Find(&listing, listingID); res.Error != nil {
		return res.Error
	} else if res.RowsAffected == 0 {
		return nil
	}

	document := listing.EmbeddingDocument()
	hash := models.ContentHash(document)

	// find-or-initialize the embedding row; Find avoids a not-found error log on
	// the common first-embed path.
	var record models.ListingEmbedding
	res := db.Where("rv_listing_id = ?", listingID).Limit(1).Find(&record)
	if res.Error != nil {
		return res.Error
	}
	persisted := res.RowsAffected > 0
	if persisted && record.ContentHash != nil && *record.ContentHash == hash {
		return nil
	}

	vec, err := embedder.Call(ctx, document, "listing_embedding", nil)
	if err != nil {
		return err
	}

	model := ai.EmbedModel
	record.RvListingID = listingID
	record.Embedding = pgvector.NewVector(vec)
	record.Document = &document
	record.Model = &model
	record.ContentHash = &hash
	return db.Save(&record).Error
}

// NewServeMux registers the task handlers a worker serves.
func NewServeMux(db *gorm.DB, embedder *ai.Embedder) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeListingEmbed, func(ctx context.Context, t *asynq.Task) error {
		var p ListingEmbedPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return EmbedListing(ctx, db, embedder, p.RvListingID)
	})
	return mux
}
