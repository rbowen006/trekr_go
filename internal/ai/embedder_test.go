//go:build integration

package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rbowen/trekr_go/internal/ai"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeOllama returns an httptest server that answers /api/embeddings with the
// given vector (mirrors the WebMock stub in embedder_spec.rb).
func fakeOllama(t *testing.T, vec []float32, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/embeddings", r.URL.Path)
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("internal error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vec})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func vec768(v float32) []float32 {
	out := make([]float32, 768)
	for i := range out {
		out[i] = v
	}
	return out
}

func seedUser(t *testing.T, db *gorm.DB, tag string) *models.User {
	t.Helper()
	u := &models.User{
		Email:             fmt.Sprintf("%s-%d@example.com", tag, testutil.UniqueID()),
		EncryptedPassword: "x",
		Name:              "Seed User",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

// lastAiRequest returns the ai_requests row for a unique feature, asserting
// exactly one exists (the shared test DB is not truncated between tests).
func lastAiRequest(t *testing.T, db *gorm.DB, feature string) models.AiRequest {
	t.Helper()
	var reqs []models.AiRequest
	require.NoError(t, db.Where("feature = ?", feature).Find(&reqs).Error)
	require.Len(t, reqs, 1)
	return reqs[0]
}

func TestEmbedder_ReturnsVectorAndLogsSuccess(t *testing.T) {
	db := testutil.OpenTestDB(t)
	want := vec768(0.01)
	srv := fakeOllama(t, want, http.StatusOK)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}
	user := seedUser(t, db, "emb-ok")
	feature := fmt.Sprintf("test-emb-ok-%d", testutil.UniqueID())

	got, err := embedder.Call(context.Background(), "Caravan in Byron Bay, NSW.", feature, &user.ID)
	require.NoError(t, err)
	require.Len(t, got, 768)
	require.InDelta(t, 0.01, got[0], 1e-6)

	rec := lastAiRequest(t, db, feature)
	require.Equal(t, "nomic-embed-text", rec.Model)
	require.True(t, rec.Success)
	require.NotNil(t, rec.EstimatedCostUSD)
	require.InDelta(t, 0.0, *rec.EstimatedCostUSD, 1e-9)
	require.NotNil(t, rec.UserID)
	require.Equal(t, user.ID, *rec.UserID)
}

func TestEmbedder_MissingEmbeddingIsError(t *testing.T) {
	db := testutil.OpenTestDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}")) // 200 but no embedding key
	}))
	t.Cleanup(srv.Close)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}
	feature := fmt.Sprintf("test-emb-missing-%d", testutil.UniqueID())

	vec, err := embedder.Call(context.Background(), "anything", feature, nil)
	require.Error(t, err)
	require.Nil(t, vec)

	rec := lastAiRequest(t, db, feature)
	require.False(t, rec.Success)
}

func TestEmbedder_ErrorRaisesAndLogsFailure(t *testing.T) {
	db := testutil.OpenTestDB(t)
	srv := fakeOllama(t, nil, http.StatusInternalServerError)
	embedder := &ai.Embedder{DB: db, OllamaURL: srv.URL, Client: srv.Client()}
	feature := fmt.Sprintf("test-emb-err-%d", testutil.UniqueID())

	_, err := embedder.Call(context.Background(), "anything", feature, nil)
	require.Error(t, err)

	rec := lastAiRequest(t, db, feature)
	require.False(t, rec.Success)
	require.NotNil(t, rec.ErrorMessage)
	require.NotEmpty(t, *rec.ErrorMessage)
}
