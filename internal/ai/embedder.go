// Package ai wraps the local AI providers (Ollama) behind small Go services,
// mirroring rv_marketplace's app/services/ai. Each call logs an ai_requests row.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

// EmbedModel is the Ollama model used for embeddings (nomic-embed-text, 768
// dims). Mirrors Ai::Embedder::MODEL.
const EmbedModel = "nomic-embed-text"

// embedFeatureCost is the estimated USD cost logged per embedding call: zero,
// since Ollama runs locally (mirrors Ai::Embedder).
const embedFeatureCost = 0.0

// Embedder calls Ollama's /api/embeddings endpoint and logs an ai_requests row
// per call (success or failure), mirroring Ai::Embedder.
type Embedder struct {
	DB        *gorm.DB
	OllamaURL string
	Client    *http.Client
}

// NewEmbedder builds an Embedder with a default HTTP client.
func NewEmbedder(db *gorm.DB, ollamaURL string) *Embedder {
	return &Embedder{DB: db, OllamaURL: ollamaURL, Client: http.DefaultClient}
}

// Call requests an embedding for text and returns the vector. It always writes
// an ai_requests row (feature, model, latency, success/error, user), even when
// the request fails — mirroring Ai::Embedder's ensure block. A non-2xx response
// or transport error is returned as an error.
func (e *Embedder) Call(ctx context.Context, text, feature string, userID *int64) (vec []float32, err error) {
	start := time.Now()
	defer func() {
		latency := int(time.Since(start).Milliseconds())
		e.writeAiRequest(feature, latency, userID, err)
	}()

	vec, err = e.requestEmbedding(ctx, text)
	return vec, err
}

func (e *Embedder) requestEmbedding(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(map[string]string{"model": EmbedModel, "prompt": text})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(e.OllamaURL, "/") + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("Ollama embeddings error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama embeddings error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Ollama embeddings error (%d): %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("Ollama embeddings error: %w", err)
	}
	// Mirror Ruby's .fetch("embedding"): a 200 with no embedding is an error,
	// not a silent nil vector (which would fail the vector(768) insert).
	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("Ollama embeddings error: response missing embedding")
	}
	return parsed.Embedding, nil
}

// writeAiRequest logs the call outcome. Best-effort: a logging failure is logged
// but never surfaced (mirrors Ai::Embedder#write_ai_request's rescue).
func (e *Embedder) writeAiRequest(feature string, latencyMs int, userID *int64, callErr error) {
	cost := embedFeatureCost
	record := &models.AiRequest{
		Feature:          feature,
		Model:            EmbedModel,
		LatencyMs:        &latencyMs,
		EstimatedCostUSD: &cost,
		Success:          callErr == nil,
		UserID:           userID,
	}
	if callErr != nil {
		msg := callErr.Error()
		record.ErrorMessage = &msg
	}
	if err := e.DB.Create(record).Error; err != nil {
		log.Printf("failed to write ai_request: %v", err)
	}
}

func (e *Embedder) client() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	return http.DefaultClient
}
