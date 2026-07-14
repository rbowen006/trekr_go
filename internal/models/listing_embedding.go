package models

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// ListingEmbedding maps to Rails' listing_embeddings table: a listing's
// semantic-search vector plus the composed document and its content_hash, used
// to skip re-embedding unchanged text (ADR-0011). The nullable columns are
// pointers so an unpopulated row round-trips as SQL NULL.
type ListingEmbedding struct {
	ID          int64           `gorm:"primaryKey"`
	RvListingID int64           `gorm:"column:rv_listing_id"`
	Embedding   pgvector.Vector `gorm:"column:embedding;type:vector(768)"`
	Document    *string         `gorm:"column:document"`
	Model       *string         `gorm:"column:model"`
	ContentHash *string         `gorm:"column:content_hash"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

func (ListingEmbedding) TableName() string { return "listing_embeddings" }
