//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pgvector/pgvector-go"
	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// unitVec returns a 768-dim vector with 1.0 at index hot, zeros elsewhere.
func unitVec(hot int) []float32 {
	v := make([]float32, 768)
	v[hot] = 1
	return v
}

// uniqueDir returns a query direction unique to this test run (a distinct hot
// index), so the ranking test's near/far are the two globally-closest rows even
// against the shared, non-truncated test DB. Returns (query==near vector, a
// slightly-off "far" vector, the hot index).
func uniqueDir(t *testing.T) (query, far []float32, hot int) {
	t.Helper()
	hot = int(testutil.UniqueID()%766) + 1 // 1..766, avoids dim 0 used elsewhere
	query = unitVec(hot)
	far = unitVec(hot)
	far[hot+1] = 0.1 // tiny off-axis component -> small positive cosine distance
	return query, far, hot
}

// stubEmbedder points the app's embedder at a fake Ollama returning the given
// query vector, and reports how many embedding requests were served.
func stubEmbedder(t *testing.T, app *httpapi.App, queryVec []float32, status int) *int {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("down"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": queryVec})
	}))
	t.Cleanup(srv.Close)
	app.Config.OllamaURL = srv.URL
	return &calls
}

func embedListingAt(t *testing.T, app *httpapi.App, listingID int64, vec []float32) {
	t.Helper()
	hash := fmt.Sprintf("h-%d", testutil.UniqueID())
	model := "nomic-embed-text"
	rec := &models.ListingEmbedding{
		RvListingID: listingID,
		Embedding:   pgvector.NewVector(vec),
		Model:       &model,
		ContentHash: &hash,
	}
	require.NoError(t, app.DB.Create(rec).Error)
}

func searchBody(query string) string {
	return fmt.Sprintf(`{"query":%q}`, query)
}

func TestSearch_RanksBySimilarityWithScore(t *testing.T) {
	app := testutil.NewTestApp(t)
	query, farVec, _ := uniqueDir(t)
	stubEmbedder(t, app, query, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "search-o")
	near := seedListing(t, app, owner.ID, "100")
	far := seedListing(t, app, owner.ID, "100")
	embedListingAt(t, app, near.ID, query) // identical to query -> distance 0
	embedListingAt(t, app, far.ID, farVec) // slightly off -> small positive distance

	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search", "", searchBody("caravan by the sea"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var results []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &results))
	require.GreaterOrEqual(t, len(results), 2)

	// near/far are the two globally-closest rows for this run's unique query
	// direction; assert their relative order (robust against the shared DB).
	nearIdx, farIdx := indexOfID(t, results, near.ID), indexOfID(t, results, far.ID)
	require.Less(t, nearIdx, farIdx, "near must rank before far")

	for _, key := range []string{"id", "title", "rv_type", "owner", "images", "score"} {
		require.Contains(t, results[nearIdx], key, "result must include %q", key)
	}
}

func TestSearch_IsPublic_AndCapsAt20(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubEmbedder(t, app, unitVec(0), http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "search-cap")
	for i := 0; i < 25; i++ {
		l := seedListing(t, app, owner.ID, "100")
		embedListingAt(t, app, l.ID, unitVec(0))
	}

	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search", "", searchBody("anything"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var results []json.RawMessage
	require.NoError(t, json.Unmarshal(body, &results))
	require.Len(t, results, 20)
}

func TestSearch_BlankQueryUnprocessable(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubEmbedder(t, app, unitVec(0), http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search", "", searchBody("   "))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestSearch_EmptyBodyIsBlankQuery422(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubEmbedder(t, app, unitVec(0), http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	// Empty body -> missing query -> 422, not 400 (matches Rails).
	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search", "", "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestSearch_IgnoresNullEmbeddingRows(t *testing.T) {
	app := testutil.NewTestApp(t)
	query, _, _ := uniqueDir(t)
	stubEmbedder(t, app, query, http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "search-null")
	// A listing_embedding row with a NULL embedding must be skipped (matches the
	// neighbor gem) rather than crashing the float64 score scan.
	nullListing := seedListing(t, app, owner.ID, "100")
	// Insert a genuine NULL embedding via raw SQL (a zero-value pgvector.Vector
	// would insert an empty vector, which pgvector rejects).
	require.NoError(t, app.DB.Exec(
		"INSERT INTO listing_embeddings (rv_listing_id, embedding, content_hash, created_at, updated_at) VALUES (?, NULL, ?, now(), now())",
		nullListing.ID, "null-emb").Error)
	hit := seedListing(t, app, owner.ID, "100")
	embedListingAt(t, app, hit.ID, query)

	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search", "", searchBody("caravan"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var results []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &results))
	for _, row := range results {
		require.NotEqual(t, nullListing.ID, idOf(t, row), "null-embedding listing must be excluded")
	}
}

func TestSearch_EmbedderDownReturns503(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubEmbedder(t, app, nil, http.StatusInternalServerError)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search", "", searchBody("caravan"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestSearch_LogsNlSearchAiRequest_AttributesAuthenticatedUser(t *testing.T) {
	app := testutil.NewTestApp(t)
	stubEmbedder(t, app, unitVec(0), http.StatusOK)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := seedOwner(t, app, "search-user")
	before := aiRequestCount(t, app, "nl_search")

	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings/search",
		testutil.AuthHeader(t, app, user), searchBody("caravan"))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, before+1, aiRequestCount(t, app, "nl_search"))

	// The most recent nl_search row belongs to the authenticated user.
	var rec models.AiRequest
	require.NoError(t, app.DB.Where("feature = ?", "nl_search").Order("id DESC").First(&rec).Error)
	require.NotNil(t, rec.UserID)
	require.Equal(t, user.ID, *rec.UserID)
}

func idOf(t *testing.T, obj map[string]json.RawMessage) int64 {
	t.Helper()
	var id int64
	require.NoError(t, json.Unmarshal(obj["id"], &id))
	return id
}

func indexOfID(t *testing.T, results []map[string]json.RawMessage, id int64) int {
	t.Helper()
	for i, r := range results {
		if idOf(t, r) == id {
			return i
		}
	}
	t.Fatalf("listing %d not found in results", id)
	return -1
}

func aiRequestCount(t *testing.T, app *httpapi.App, feature string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, app.DB.Model(&models.AiRequest{}).Where("feature = ?", feature).Count(&n).Error)
	return n
}
