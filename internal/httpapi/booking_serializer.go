package httpapi

import "github.com/rbowen/trekr_go/internal/models"

// bookingJSON matches Booking#as_json (default, no participants): the field set
// and key order rendered by create/confirm/reject.
type bookingJSON struct {
	ID          int64  `json:"id"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Status      string `json:"status"`
	HirerID     int64  `json:"hirer_id"`
	RvListingID int64  `json:"rv_listing_id"`
}

// bookingParticipantsJSON matches Booking#as_json(include_participants: true),
// used by index and show. trip_planning_available is present only on show
// (nil => omitted), matching the controller.
type bookingParticipantsJSON struct {
	ID                    int64     `json:"id"`
	StartDate             string    `json:"start_date"`
	EndDate               string    `json:"end_date"`
	Status                string    `json:"status"`
	HirerID               int64     `json:"hirer_id"`
	RvListingID           int64     `json:"rv_listing_id"`
	Hirer                 ownerJSON `json:"hirer"`
	Owner                 ownerJSON `json:"owner"`
	ListingTitle          string    `json:"listing_title"`
	TripPlanningAvailable *bool     `json:"trip_planning_available,omitempty"`
}

const bookingDateLayout = "2006-01-02"

func serializeBooking(b *models.Booking) bookingJSON {
	return bookingJSON{
		ID:          b.ID,
		StartDate:   b.StartDate.Format(bookingDateLayout),
		EndDate:     b.EndDate.Format(bookingDateLayout),
		Status:      b.Status,
		HirerID:     b.HirerID,
		RvListingID: b.RvListingID,
	}
}

// serializeBookingParticipants builds the participants payload. tripAvailable is
// nil for index (field omitted) and a pointer for show.
func serializeBookingParticipants(b *models.Booking, tripAvailable *bool) bookingParticipantsJSON {
	out := bookingParticipantsJSON{
		ID:                    b.ID,
		StartDate:             b.StartDate.Format(bookingDateLayout),
		EndDate:               b.EndDate.Format(bookingDateLayout),
		Status:                b.Status,
		HirerID:               b.HirerID,
		RvListingID:           b.RvListingID,
		ListingTitle:          b.RvListing.Title,
		TripPlanningAvailable: tripAvailable,
	}
	if b.Hirer != nil {
		out.Hirer = ownerJSON{ID: b.Hirer.ID, Name: b.Hirer.Name}
	}
	if b.RvListing != nil && b.RvListing.Owner != nil {
		out.Owner = ownerJSON{ID: b.RvListing.Owner.ID, Name: b.RvListing.Owner.Name}
	}
	return out
}
