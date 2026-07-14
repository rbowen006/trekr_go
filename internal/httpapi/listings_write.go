package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/region"
	"github.com/rbowen/trekr_go/internal/services"
	"gorm.io/gorm"
)

// regionsOnce loads and caches the embedded region manifest. The manifest is a
// build-time artifact, so parse it once per process (MustLoad panics only if
// the embedded manifest is malformed — a build bug).
var regionsOnce = sync.OnceValue(region.MustLoad)

// listingService builds a ListingService wired to this app's dependencies.
func (app *App) listingService() *services.ListingService {
	return &services.ListingService{
		DB:          app.DB,
		StorageRoot: app.Config.StorageRoot,
		Regions:     regionsOnce(),
		Queue:       app.EmbedQueue,
	}
}

// listingsMine serves GET /api/v1/listings/mine — the current user's listings.
func (app *App) listingsMine(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	var listings []models.RvListing
	// No ORDER BY, matching current_user.rv_listings' natural scan order.
	if err := app.DB.Preload("Owner").Where("owner_id = ?", user.ID).Find(&listings).Error; err != nil {
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

// listingsCreate serves POST /api/v1/listings.
func (app *App) listingsCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	params, images, err := parseListingRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	listing, validationErrors, err := app.listingService().Create(user.ID, params, images)
	if err != nil {
		http.Error(w, "could not create listing", http.StatusInternalServerError)
		return
	}
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}
	app.renderListing(w, http.StatusCreated, listing.ID)
}

// listingsUpdate serves PUT/PATCH /api/v1/listings/:id (owner only).
func (app *App) listingsUpdate(w http.ResponseWriter, r *http.Request) {
	listing, ok := app.findOwnedListing(w, r, "id")
	if !ok {
		return
	}

	params, _, err := parseListingRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	validationErrors, err := app.listingService().Update(listing, params)
	if err != nil {
		http.Error(w, "could not update listing", http.StatusInternalServerError)
		return
	}
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}
	app.renderListing(w, http.StatusOK, listing.ID)
}

// listingsDestroy serves DELETE /api/v1/listings/:id (owner only).
func (app *App) listingsDestroy(w http.ResponseWriter, r *http.Request) {
	listing, ok := app.findOwnedListing(w, r, "id")
	if !ok {
		return
	}
	if err := app.listingService().Destroy(listing); err != nil {
		http.Error(w, "could not delete listing", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findOwnedListing loads the listing named by the given URL param and enforces
// ownership, writing the 404/403 response itself. It mirrors the Rails
// set_listing + authorize_owner! before_actions (404 before 403). ok is false
// when a response has already been written.
func (app *App) findOwnedListing(w http.ResponseWriter, r *http.Request, idParam string) (*models.RvListing, bool) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return nil, false
	}

	id, err := strconv.ParseInt(chi.URLParam(r, idParam), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return nil, false
	}

	var listing models.RvListing
	err = app.DB.First(&listing, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return nil, false
	}
	if err != nil {
		http.Error(w, "could not load listing", http.StatusInternalServerError)
		return nil, false
	}

	if listing.OwnerID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not authorized"})
		return nil, false
	}
	return &listing, true
}

// renderListing reloads a listing with its owner and writes the Rails-compatible
// JSON, so responses reflect DB-normalized values (id, region, price format).
func (app *App) renderListing(w http.ResponseWriter, status int, id int64) {
	var listing models.RvListing
	if err := app.DB.Preload("Owner").First(&listing, id).Error; err != nil {
		http.Error(w, "could not load listing", http.StatusInternalServerError)
		return
	}
	out, err := app.serializeListings(app.DB, []models.RvListing{listing})
	if err != nil {
		http.Error(w, "could not serialize listing", http.StatusInternalServerError)
		return
	}
	writeJSON(w, status, out[0])
}
