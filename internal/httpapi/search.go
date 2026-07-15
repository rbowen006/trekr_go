package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rbowen/trekr_go/internal/ai"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/services"
)

func (app *App) embedder() *ai.Embedder {
	return ai.NewEmbedder(app.DB, app.Config.OllamaURL)
}

func (app *App) searchService() *services.SearchService {
	return &services.SearchService{DB: app.DB}
}

// scoredListingJSON is a listing payload with the appended semantic-search
// score (cosine distance), matching `rv_listing.as_json.merge('score' => ...)`:
// listing fields first (embedded), then score.
type scoredListingJSON struct {
	listingJSON
	Score float64 `json:"score"`
}

type searchRequest struct {
	Query string `json:"query"`
}

// listingsSearch serves POST /api/v1/listings/search — public natural-language
// search. It embeds the query (attributing the caller when a token is present),
// runs the pgvector nearest-neighbour query, and returns listings ranked by
// similarity, each with a score. Mirrors ListingsController#search.
func (app *App) listingsSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	// Malformed (non-empty invalid) JSON is rejected upstream by the
	// MalformedJSON middleware; an empty body (io.EOF) is treated as a missing
	// query, which the blank check below turns into a 422 (matching Rails'
	// params[:query].to_s.strip).
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "query is required"})
		return
	}

	var userID *int64
	if user, ok := mw.CurrentUser(r.Context()); ok {
		userID = &user.ID
	}

	// An embedder failure is a service outage (Rails rescues Ai::ApiError => 503).
	vec, err := app.embedder().Call(r.Context(), query, "nl_search", userID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": err.Error()})
		return
	}

	scored, err := app.searchService().Nearest(vec)
	if err != nil {
		http.Error(w, "could not search listings", http.StatusInternalServerError)
		return
	}

	listings := make([]models.RvListing, len(scored))
	for i, s := range scored {
		listings[i] = s.Listing
	}
	serialized, err := app.serializeListings(app.DB, listings)
	if err != nil {
		http.Error(w, "could not serialize listings", http.StatusInternalServerError)
		return
	}

	out := make([]scoredListingJSON, len(serialized))
	for i := range serialized {
		out[i] = scoredListingJSON{listingJSON: serialized[i], Score: scored[i].Score}
	}
	writeJSON(w, http.StatusOK, out)
}
