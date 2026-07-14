package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/services"
	"gorm.io/gorm"
)

func (app *App) bookingService() *services.BookingService {
	return &services.BookingService{DB: app.DB}
}

type bookingParamsRequest struct {
	Booking struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	} `json:"booking"`
}

// bookingsCreate serves POST /api/v1/listings/:listing_id/bookings.
func (app *App) bookingsCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "listing_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}
	var listing models.RvListing
	if err := app.DB.First(&listing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
		http.Error(w, "could not load listing", http.StatusInternalServerError)
		return
	}

	if listing.OwnerID == user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Owners cannot book their own listing"})
		return
	}

	var req bookingParamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "fail", "message": "Malformed request body"})
		return
	}

	booking, validationErrors, err := app.bookingService().Create(user.ID, &listing, req.Booking.StartDate, req.Booking.EndDate)
	if err != nil {
		http.Error(w, "could not create booking", http.StatusInternalServerError)
		return
	}
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": validationErrors})
		return
	}
	writeJSON(w, http.StatusCreated, serializeBooking(booking))
}

// bookingsIndex serves GET /api/v1/bookings — bookings the user hires or owns,
// newest first, with participant details.
func (app *App) bookingsIndex(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	var bookings []models.Booking
	err := app.DB.
		// Qualify the select so the manual join's rv_listings columns (id,
		// created_at) don't collide with bookings' during scan.
		Select("bookings.*").
		Preload("Hirer").Preload("RvListing.Owner").
		Joins("JOIN rv_listings ON rv_listings.id = bookings.rv_listing_id").
		Where("bookings.hirer_id = ? OR rv_listings.owner_id = ?", user.ID, user.ID).
		Order("bookings.created_at DESC").
		Find(&bookings).Error
	if err != nil {
		http.Error(w, "could not load bookings", http.StatusInternalServerError)
		return
	}

	out := make([]bookingParticipantsJSON, len(bookings))
	for i := range bookings {
		// Index omits trip_planning_available (nil), matching Rails.
		out[i] = serializeBookingParticipants(&bookings[i], nil)
	}
	writeJSON(w, http.StatusOK, out)
}

// bookingsShow serves GET /api/v1/bookings/:id — participants only; 404 for a
// missing booking or a non-participant (mirrors the controller).
func (app *App) bookingsShow(w http.ResponseWriter, r *http.Request) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	booking, ok := app.loadBooking(w, r)
	if !ok {
		return
	}

	if booking.HirerID != user.ID && booking.RvListing.OwnerID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}

	available := app.bookingService().TripPlanningAvailable(booking, booking.RvListing)
	writeJSON(w, http.StatusOK, serializeBookingParticipants(booking, &available))
}

// bookingsConfirm serves PATCH /api/v1/bookings/:id/confirm (listing owner only).
func (app *App) bookingsConfirm(w http.ResponseWriter, r *http.Request) {
	app.transitionBooking(w, r, "confirmed", "Only listing owner can confirm")
}

// bookingsReject serves PATCH /api/v1/bookings/:id/reject (listing owner only).
func (app *App) bookingsReject(w http.ResponseWriter, r *http.Request) {
	app.transitionBooking(w, r, "rejected", "Only listing owner can reject")
}

func (app *App) transitionBooking(w http.ResponseWriter, r *http.Request, status, forbidMsg string) {
	user, ok := mw.CurrentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "fail", "message": "Unauthorized"})
		return
	}

	booking, ok := app.loadBooking(w, r)
	if !ok {
		return
	}
	if booking.RvListing.OwnerID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": forbidMsg})
		return
	}

	if err := app.bookingService().SetStatus(booking, status); err != nil {
		http.Error(w, "could not update booking", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, serializeBooking(booking))
}

// loadBooking loads the booking named by {id} with its hirer and listing owner.
// It writes a 404 and returns ok=false when the booking is missing.
func (app *App) loadBooking(w http.ResponseWriter, r *http.Request) (*models.Booking, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return nil, false
	}
	var booking models.Booking
	err = app.DB.Preload("Hirer").Preload("RvListing.Owner").First(&booking, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return nil, false
	}
	if err != nil {
		http.Error(w, "could not load booking", http.StatusInternalServerError)
		return nil, false
	}
	return &booking, true
}
