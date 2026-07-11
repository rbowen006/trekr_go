package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
)

// listingsIndex serves the public GET /api/v1/listings.
func (app *App) listingsIndex(w http.ResponseWriter, r *http.Request) {
	var listings []models.RvListing
	// No ORDER BY, matching Rails' RvListing.all: both read Postgres' natural
	// scan order so the array order stays identical between backends.
	if err := app.DB.Preload("Owner").Find(&listings).Error; err != nil {
		http.Error(w, "could not load listings", http.StatusInternalServerError)
		return
	}
	out, err := app.serializeListings(app.DB, listings)
	if err != nil {
		http.Error(w, "could not serialize listings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// listingsShow serves the public GET /api/v1/listings/:id.
func (app *App) listingsShow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return
	}

	var listing models.RvListing
	err = app.DB.Preload("Owner").First(&listing, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return
	}
	if err != nil {
		http.Error(w, "could not load listing", http.StatusInternalServerError)
		return
	}

	out, err := app.serializeListings(app.DB, []models.RvListing{listing})
	if err != nil {
		http.Error(w, "could not serialize listing", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, out[0])
}
