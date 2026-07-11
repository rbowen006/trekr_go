package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// imagesCreate serves POST /api/v1/listings/:listing_id/images (owner only):
// attaches uploaded images and returns the updated listing. Mirrors
// ImagesController#create.
func (app *App) imagesCreate(w http.ResponseWriter, r *http.Request) {
	listing, ok := app.findOwnedListing(w, r, "listing_id")
	if !ok {
		return
	}

	images, err := parseImageUpload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}
	if err := app.listingService().AttachImages(listing.ID, images); err != nil {
		http.Error(w, "could not attach images", http.StatusInternalServerError)
		return
	}
	app.renderListing(w, http.StatusCreated, listing.ID)
}

// imagesDestroy serves DELETE /api/v1/listings/:listing_id/images/:id (owner
// only): purges the attachment and returns 204. Mirrors ImagesController#destroy
// (attachment.purge_later); a missing attachment 404s like RecordNotFound.
func (app *App) imagesDestroy(w http.ResponseWriter, r *http.Request) {
	listing, ok := app.findOwnedListing(w, r, "listing_id")
	if !ok {
		return
	}

	attachmentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return
	}

	found, err := app.listingService().DeleteImage(listing.ID, attachmentID)
	if err != nil {
		http.Error(w, "could not delete image", http.StatusInternalServerError)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
