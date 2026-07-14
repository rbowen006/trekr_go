//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/jobs"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// countingOllama returns a fake /api/embeddings server plus a counter of the
// requests it served, so tests can assert re-embed behaviour (mirrors
// have_been_made.once/twice/not).
func countingOllama(t *testing.T, v float32) (*httptest.Server, *int32) {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		vec := make([]float32, 768)
		for i := range vec {
			vec[i] = v
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vec})
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

func seedUser(t *testing.T, db *gorm.DB) *models.User {
	t.Helper()
	u := &models.User{
		Email:             fmt.Sprintf("embj-%d@example.com", testutil.UniqueID()),
		EncryptedPassword: "x",
		Name:              "Seed User",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

func seedListing(t *testing.T, db *gorm.DB, ownerID int64) *models.RvListing {
	t.Helper()
	price := "100"
	l := &models.RvListing{
		Title:       "Cozy Caravan",
		Description: "Lovely",
		PricePerDay: &price,
		OwnerID:     ownerID,
		MaxGuests:   4,
		RvType:      0,
		Town:        "Byron Bay",
		State:       "NSW",
		Postcode:    "2481",
	}
	require.NoError(t, db.Create(l).Error)
	return l
}

func embeddingFor(t *testing.T, db *gorm.DB, listingID int64) (models.ListingEmbedding, bool) {
	t.Helper()
	var rec models.ListingEmbedding
	err := db.Where("rv_listing_id = ?", listingID).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return rec, false
	}
	require.NoError(t, err)
	return rec, true
}

func TestEmbedListing_EmbedsAndStores(t *testing.T) {
	db := testutil.OpenTestDB(t)
	srv, _ := countingOllama(t, 0.02)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}
	listing := seedListing(t, db, seedUser(t, db).ID)

	require.NoError(t, jobs.EmbedListing(context.Background(), db, embedder, listing.ID))

	rec, ok := embeddingFor(t, db, listing.ID)
	require.True(t, ok)
	require.Len(t, rec.Embedding.Slice(), 768)
	require.NotNil(t, rec.Document)
	require.Equal(t, listing.EmbeddingDocument(), *rec.Document)
	require.NotNil(t, rec.Model)
	require.Equal(t, "nomic-embed-text", *rec.Model)
	require.NotNil(t, rec.ContentHash)
	require.Equal(t, models.ContentHash(listing.EmbeddingDocument()), *rec.ContentHash)
}

func TestEmbedListing_SkipsWhenDocumentUnchanged(t *testing.T) {
	db := testutil.OpenTestDB(t)
	srv, count := countingOllama(t, 0.02)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}
	listing := seedListing(t, db, seedUser(t, db).ID)

	require.NoError(t, jobs.EmbedListing(context.Background(), db, embedder, listing.ID))
	first, _ := embeddingFor(t, db, listing.ID)

	require.NoError(t, jobs.EmbedListing(context.Background(), db, embedder, listing.ID))
	second, _ := embeddingFor(t, db, listing.ID)

	require.Equal(t, int32(1), atomic.LoadInt32(count), "should embed only once")
	require.Equal(t, first.UpdatedAt, second.UpdatedAt, "unchanged document must not re-embed")
}

func TestEmbedListing_ReembedsWhenDocumentChanges(t *testing.T) {
	db := testutil.OpenTestDB(t)
	srv, count := countingOllama(t, 0.02)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}
	listing := seedListing(t, db, seedUser(t, db).ID)

	require.NoError(t, jobs.EmbedListing(context.Background(), db, embedder, listing.ID))
	require.NoError(t, db.Model(listing).Update("title", "A brand new headline").Error)
	require.NoError(t, jobs.EmbedListing(context.Background(), db, embedder, listing.ID))

	require.Equal(t, int32(2), atomic.LoadInt32(count), "changed document must re-embed")
	rec, _ := embeddingFor(t, db, listing.ID)
	require.NotNil(t, rec.Document)
	require.Contains(t, *rec.Document, "A brand new headline")
}

func TestEmbedListing_NoopWhenListingMissing(t *testing.T) {
	db := testutil.OpenTestDB(t)
	srv, count := countingOllama(t, 0.02)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}

	require.NoError(t, jobs.EmbedListing(context.Background(), db, embedder, -1))
	require.Equal(t, int32(0), atomic.LoadInt32(count), "missing listing must not call Ollama")
}
