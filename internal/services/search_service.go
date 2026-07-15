package services

import (
	"github.com/pgvector/pgvector-go"
	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

// searchLimit caps semantic-search results, mirroring SEARCH_LIMIT.
const searchLimit = 20

// ScoredListing pairs a listing with its cosine distance to the query embedding
// (smaller is nearer), mirroring `rv_listing.as_json.merge('score' => distance)`.
type ScoredListing struct {
	Listing models.RvListing
	Score   float64
}

// SearchService runs the pgvector nearest-neighbour query for semantic search.
// The embedding call lives in the handler so an embedder failure maps to 503
// while a query failure maps to 500 (mirroring ListingsController#search's
// rescue Ai::ApiError).
type SearchService struct {
	DB *gorm.DB
}

// Nearest returns the listings whose embeddings are closest to queryVec by
// cosine distance, nearest first, capped at searchLimit, with owners preloaded
// for serialization. Mirrors ListingEmbedding.nearest_neighbors(distance:
// :cosine).limit(SEARCH_LIMIT).
func (s *SearchService) Nearest(queryVec []float32) ([]ScoredListing, error) {
	qv := pgvector.NewVector(queryVec)

	var rows []struct {
		RvListingID int64
		Score       float64
	}
	// `<=>` is pgvector's cosine-distance operator; order by the alias so the
	// vector is bound once. `::vector` casts the text-encoded bind parameter.
	// Exclude null embeddings, matching the neighbor gem's
	// nearest_neighbors (`.where.not(embedding: nil)`) — a null vector would
	// yield a NULL score that fails the float64 scan.
	err := s.DB.Model(&models.ListingEmbedding{}).
		Select("rv_listing_id, (embedding <=> ?::vector) AS score", qv).
		Where("embedding IS NOT NULL").
		Order("score ASC").
		Limit(searchLimit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []ScoredListing{}, nil
	}

	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.RvListingID
	}

	var listings []models.RvListing
	if err := s.DB.Preload("Owner").Where("id IN ?", ids).Find(&listings).Error; err != nil {
		return nil, err
	}
	byID := make(map[int64]models.RvListing, len(listings))
	for _, l := range listings {
		byID[l.ID] = l
	}

	// Preserve nearest-first order; skip any embedding whose listing has since
	// been deleted (Rails would have cascaded the embedding away, so this is
	// defensive).
	out := make([]ScoredListing, 0, len(rows))
	for _, r := range rows {
		if l, ok := byID[r.RvListingID]; ok {
			out = append(out, ScoredListing{Listing: l, Score: r.Score})
		}
	}
	return out, nil
}
