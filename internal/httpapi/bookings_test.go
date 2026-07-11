//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func futureDate(days int) string {
	return time.Now().UTC().AddDate(0, 0, days).Format("2006-01-02")
}

func seedBooking(t *testing.T, app *httpapi.App, listingID, hirerID int64, status string, startDays, endDays int) *models.Booking {
	t.Helper()
	b := &models.Booking{
		StartDate:   time.Now().UTC().AddDate(0, 0, startDays),
		EndDate:     time.Now().UTC().AddDate(0, 0, endDays),
		Status:      status,
		HirerID:     hirerID,
		RvListingID: listingID,
	}
	require.NoError(t, app.DB.Create(b).Error)
	return b
}

func bookingBody(startDays, endDays int) string {
	return fmt.Sprintf(`{"booking":{"start_date":%q,"end_date":%q}}`, futureDate(startDays), futureDate(endDays))
}

// --- POST /api/v1/listings/:listing_id/bookings ---------------------------

func TestBookingCreate_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-auth")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID), "", bookingBody(1, 3))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestBookingCreate_OwnerForbidden(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-own")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID),
		testutil.AuthHeader(t, app, owner), bookingBody(1, 3))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestBookingCreate_HirerSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-o")
	hirer := seedOwner(t, app, "bk-h")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID),
		testutil.AuthHeader(t, app, hirer), bookingBody(1, 3))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		ID          int64 `json:"id"`
		HirerID     int64 `json:"hirer_id"`
		RvListingID int64 `json:"rv_listing_id"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	// Byte-exact default shape (field set + order) vs Rails.
	want := fmt.Sprintf(`{"id":%d,"start_date":%q,"end_date":%q,"status":"pending","hirer_id":%d,"rv_listing_id":%d}`,
		got.ID, futureDate(1), futureDate(3), hirer.ID, listing.ID)
	require.Equal(t, want, string(body))
	require.Equal(t, hirer.ID, got.HirerID)
}

func TestBookingCreate_Overlap_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-ovo")
	hirer1 := seedOwner(t, app, "bk-ov1")
	hirer2 := seedOwner(t, app, "bk-ov2")
	listing := seedListing(t, app, owner.ID, "100")
	seedBooking(t, app, listing.ID, hirer1.ID, "pending", 5, 7)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID),
		testutil.AuthHeader(t, app, hirer2), bookingBody(6, 8))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var got struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Contains(t, got.Errors, "Booking dates overlap with existing booking")
}

// Adjacency (new start == existing end) counts as overlap (edge-case spec).
func TestBookingCreate_AdjacentCountsAsOverlap(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-adjo")
	hirer1 := seedOwner(t, app, "bk-adj1")
	hirer2 := seedOwner(t, app, "bk-adj2")
	listing := seedListing(t, app, owner.ID, "100")
	seedBooking(t, app, listing.ID, hirer1.ID, "pending", 10, 12)

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID),
		testutil.AuthHeader(t, app, hirer2), bookingBody(12, 14))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestBookingCreate_MissingDates_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-mo")
	hirer := seedOwner(t, app, "bk-mh")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID),
		testutil.AuthHeader(t, app, hirer), `{"booking":{}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var got struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, []string{"Start date can't be blank", "End date can't be blank"}, got.Errors)
}

func TestBookingCreate_PastDate_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "bk-po")
	hirer := seedOwner(t, app, "bk-ph")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/listings/%d/bookings", server.URL, listing.ID),
		testutil.AuthHeader(t, app, hirer), bookingBody(-3, 3))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var got struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Contains(t, got.Errors, "Start date cannot be in the past")
}

// --- PATCH confirm / reject -----------------------------------------------

func TestBookingConfirm_NonOwnerForbidden(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "cf-o")
	hirer := seedOwner(t, app, "cf-h")
	listing := seedListing(t, app, owner.ID, "100")
	booking := seedBooking(t, app, listing.ID, hirer.ID, "pending", 1, 3)

	resp := doAuthJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/bookings/%d/confirm", server.URL, booking.ID),
		testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestBookingConfirm_OwnerSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "cf2-o")
	hirer := seedOwner(t, app, "cf2-h")
	listing := seedListing(t, app, owner.ID, "100")
	booking := seedBooking(t, app, listing.ID, hirer.ID, "pending", 1, 3)

	resp := doAuthJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/bookings/%d/confirm", server.URL, booking.ID),
		testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "confirmed", got.Status)
}

func TestBookingReject_OwnerSucceeds(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "rj-o")
	hirer := seedOwner(t, app, "rj-h")
	listing := seedListing(t, app, owner.ID, "100")
	booking := seedBooking(t, app, listing.ID, hirer.ID, "pending", 5, 7)

	resp := doAuthJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/bookings/%d/reject", server.URL, booking.ID),
		testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "rejected", got.Status)
}

// --- GET index / show ------------------------------------------------------

func TestBookingsIndex_ParticipantShape(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "ix-o")
	hirer := seedOwner(t, app, "ix-h")
	listing := seedListing(t, app, owner.ID, "100")
	booking := seedBooking(t, app, listing.ID, hirer.ID, "pending", 1, 3)

	resp := doAuthJSON(t, http.MethodGet, server.URL+"/api/v1/bookings", testutil.AuthHeader(t, app, hirer), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `"listing_title"`)
	require.Contains(t, string(body), `"hirer":{"id":`)
	require.Contains(t, string(body), `"owner":{"id":`)
	// index omits trip_planning_available
	require.NotContains(t, string(body), "trip_planning_available")

	var got []struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	ids := map[int64]bool{}
	for _, b := range got {
		ids[b.ID] = true
	}
	require.True(t, ids[booking.ID])
}

func TestBookingShow_ParticipantIncludesTripFlag(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "sh-o")
	hirer := seedOwner(t, app, "sh-h")
	listing := seedListing(t, app, owner.ID, "100")
	booking := seedBooking(t, app, listing.ID, hirer.ID, "confirmed", 1, 3)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/bookings/%d", server.URL, booking.ID),
		testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `"trip_planning_available":false`)
	require.True(t, strings.Contains(string(body), `"listing_title"`))
}

func TestBookingShow_NonParticipant_404(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "np-o")
	hirer := seedOwner(t, app, "np-h")
	stranger := seedOwner(t, app, "np-s")
	listing := seedListing(t, app, owner.ID, "100")
	booking := seedBooking(t, app, listing.ID, hirer.ID, "pending", 1, 3)

	resp := doAuthJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/bookings/%d", server.URL, booking.ID),
		testutil.AuthHeader(t, app, stranger), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
