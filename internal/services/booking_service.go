package services

import (
	"errors"
	"time"

	"github.com/rbowen/trekr_go/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// dateLayout is the "YYYY-MM-DD" format Rails uses for date params and JSON.
const dateLayout = "2006-01-02"

// activeBookingStatuses are the statuses that block overlapping dates
// (Booking#no_date_overlap).
var activeBookingStatuses = []string{"pending", "confirmed"}

// errBookingInvalid rolls back the create transaction when validation fails,
// without being surfaced as an internal error.
var errBookingInvalid = errors.New("booking invalid")

// BookingService owns booking creation (with overlap validation under a row
// lock) and status transitions. Mirrors BookingsController + the Booking model.
type BookingService struct {
	DB *gorm.DB
}

// Create validates and persists a pending booking for the given listing/hirer.
// It mirrors the Booking model validations (presence, end-after-start,
// start-not-in-past, no-date-overlap), running the overlap check under a
// SELECT ... FOR UPDATE lock on the listing's bookings so concurrent attempts
// serialize (BookingsController#create's `bookings.lock(true)`). Returns the
// booking, Rails-style validation messages (=> 422), or an internal error.
func (s *BookingService) Create(hirerID int64, listing *models.RvListing, startStr, endStr string) (*models.Booking, []string, error) {
	start, startOK := parseDate(startStr)
	end, endOK := parseDate(endStr)

	var errs []string
	if !startOK {
		errs = append(errs, "Start date can't be blank")
	}
	if !endOK {
		errs = append(errs, "End date can't be blank")
	}
	if startOK && endOK && !end.After(start) {
		errs = append(errs, "End date must be after start date")
	}
	if startOK && start.Before(today()) {
		errs = append(errs, "Start date cannot be in the past")
	}

	var booking *models.Booking
	txErr := s.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the listing's existing bookings so a concurrent create sees this
		// one before its own overlap check (mirrors bookings.lock(true)).
		var locked []models.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("rv_listing_id = ?", listing.ID).Find(&locked).Error; err != nil {
			return err
		}

		if startOK && endOK {
			overlaps, err := hasOverlap(tx, listing.ID, start, end)
			if err != nil {
				return err
			}
			if overlaps {
				errs = append(errs, "Booking dates overlap with existing booking")
			}
		}

		if len(errs) > 0 {
			return errBookingInvalid
		}

		booking = &models.Booking{
			StartDate:   start,
			EndDate:     end,
			Status:      "pending",
			HirerID:     hirerID,
			RvListingID: listing.ID,
		}
		return tx.Create(booking).Error
	})

	if errors.Is(txErr, errBookingInvalid) {
		return nil, errs, nil
	}
	if txErr != nil {
		return nil, nil, txErr
	}
	return booking, nil, nil
}

// SetStatus transitions a booking's status (confirm/reject) and persists it.
func (s *BookingService) SetStatus(booking *models.Booking, status string) error {
	booking.Status = status
	return s.DB.Model(booking).Update("status", status).Error
}

// TripPlanningAvailable mirrors Booking#trip_planning_available? (ADR-0013):
// true only for a confirmed booking whose region has an embedded corpus (at
// least one knowledge_chunk), so every offered plan is grounded.
func (s *BookingService) TripPlanningAvailable(booking *models.Booking, listing *models.RvListing) bool {
	if booking.Status != "confirmed" || listing == nil || listing.Region == nil || *listing.Region == "" {
		return false
	}
	var count int64
	if err := s.DB.Table("knowledge_chunks").Where("region = ?", *listing.Region).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// hasOverlap reports whether an active booking on the listing overlaps [start,
// end]. Mirrors Booking#no_date_overlap's `NOT (end_date < s OR start_date > e)`
// — adjacency (end_date == start_date) counts as overlap.
func hasOverlap(tx *gorm.DB, listingID int64, start, end time.Time) (bool, error) {
	var count int64
	err := tx.Model(&models.Booking{}).
		Where("rv_listing_id = ? AND status IN ?", listingID, activeBookingStatuses).
		Where("NOT (end_date < ? OR start_date > ?)", start, end).
		Count(&count).Error
	return count > 0, err
}

// parseDate parses a "YYYY-MM-DD" date at UTC midnight. ok is false for an
// empty or unparseable value, which the model treats as a blank date.
func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// today returns the current date at UTC midnight, for the start-not-in-past
// check (Rails compares against Date.current).
func today() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}
