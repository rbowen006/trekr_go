package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rbowen/trekr_go/internal/ai"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
)

// generateDescriptionRequest mirrors the description generator params. Pointers
// distinguish an omitted field from a zero value so ValidateInput can tell a
// missing required field from a present-but-empty one (parity with Rails'
// blank? checks).
type generateDescriptionRequest struct {
	RvType      *string  `json:"rv_type"`
	Town        *string  `json:"town"`
	State       *string  `json:"state"`
	MaxGuests   *int     `json:"max_guests"`
	PetFriendly *bool    `json:"pet_friendly"`
	PricePerDay *float64 `json:"price_per_day"`
}

// generateDescription serves POST /api/v1/listings/generate_description — an
// authed, rate-limited call that asks Claude for a listing description. Mirrors
// Api::V1::DescriptionGeneratorController#create.
func (app *App) generateDescription(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	var req generateDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	in := ai.DescriptionInput{
		MaxGuests:   req.MaxGuests,
		PricePerDay: req.PricePerDay,
	}
	if req.RvType != nil {
		in.RvType = *req.RvType
	}
	if req.Town != nil {
		in.Town = *req.Town
	}
	if req.State != nil {
		in.State = *req.State
	}
	if req.PetFriendly != nil {
		in.PetFriendly = *req.PetFriendly
	}

	data, err := app.Claude.GenerateDescription(r.Context(), in, &user.ID)
	if err != nil {
		writeAiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "data": data})
}

// writeAiError maps the three Ai error categories to their HTTP responses,
// mirroring the rescue clauses shared by both AI controllers: InputError -> 400
// (fail), ApiError -> 503 (error), OutputError -> 500 (error).
func writeAiError(w http.ResponseWriter, err error) {
	var inputErr *ai.InputError
	var apiErr *ai.ApiError
	var outputErr *ai.OutputError
	switch {
	case errors.As(err, &inputErr):
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": err.Error()})
	case errors.As(err, &apiErr):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": err.Error()})
	case errors.As(err, &outputErr):
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": err.Error()})
	}
}
