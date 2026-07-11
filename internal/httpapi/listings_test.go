//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/storage"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func seedListing(t *testing.T, app *httpapi.App, ownerID int64, price string) *models.RvListing {
	t.Helper()
	listing := &models.RvListing{
		Title:       "Cozy Caravan",
		Description: "Lovely",
		PricePerDay: &price,
		OwnerID:     ownerID,
		MaxGuests:   4,
		PetFriendly: false,
		RvType:      0,
		Town:        "Byron Bay",
		State:       "NSW",
		Postcode:    "2481",
	}
	require.NoError(t, app.DB.Create(listing).Error)
	return listing
}

// Byte-exact assertion: locks the field set, key order, rv_type enum string,
// BigDecimal price formatting, owner nesting, and empty images. The same shape
// was verified byte-for-byte against the real Rails API.
func TestListingsShow_MatchesRailsShape(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := testutil.SeedUser(t, app, fmt.Sprintf("owner-%d@example.com", testutil.UniqueID()), "Password123!")
	listing := seedListing(t, app, owner.ID, "150")

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))

	body, _ := io.ReadAll(resp.Body)
	want := fmt.Sprintf(
		`{"id":%d,"title":"Cozy Caravan","description":"Lovely","rv_type":"caravan","town":"Byron Bay","state":"NSW","postcode":"2481","price_per_day":"150.0","owner_id":%d,"max_guests":4,"pet_friendly":false,"latitude":null,"longitude":null,"owner":{"id":%d,"name":"Seed User"},"images":[]}`,
		listing.ID, owner.ID, owner.ID,
	)
	require.Equal(t, want, string(body))
}

func TestListingsShow_WithImage_URLDecodesToBlob(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := testutil.SeedUser(t, app, fmt.Sprintf("owner-img-%d@example.com", testutil.UniqueID()), "Password123!")
	listing := seedListing(t, app, owner.ID, "200")
	blob, attachment := seedListingImage(t, app, listing.ID, "photo.jpg")

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Images []struct {
			ID  int64  `json:"id"`
			URL string `json:"url"`
		} `json:"images"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Images, 1)
	require.Equal(t, attachment.ID, got.Images[0].ID, "image id is the attachment id")

	prefix := "/rails/active_storage/blobs/redirect/"
	require.True(t, strings.HasPrefix(got.Images[0].URL, prefix))
	rest := strings.TrimPrefix(got.Images[0].URL, prefix)
	signedID := rest[:strings.LastIndex(rest, "/")]
	require.True(t, strings.HasSuffix(rest, "/photo.jpg"))

	blobID, err := storage.VerifySignedID(app.Config.SecretKeyBase, signedID)
	require.NoError(t, err)
	require.Equal(t, blob.ID, blobID, "url signed id decodes to the blob id")
}

func TestListingsIndex_Public_IncludesListing(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := testutil.SeedUser(t, app, fmt.Sprintf("owner-idx-%d@example.com", testutil.UniqueID()), "Password123!")
	listing := seedListing(t, app, owner.ID, "175")

	// No Authorization header — index must be public.
	resp, err := http.Get(server.URL + "/api/v1/listings")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listings []struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listings))
	ids := make([]int64, len(listings))
	for i, l := range listings {
		ids[i] = l.ID
	}
	require.Contains(t, ids, listing.ID)
}

func TestListingsShow_NotFound_Returns404(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/api/v1/listings/999999999")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func seedListingImage(t *testing.T, app *httpapi.App, listingID int64, filename string) (*models.ActiveStorageBlob, *models.ActiveStorageAttachment) {
	t.Helper()
	key := fmt.Sprintf("k%027d", testutil.UniqueID())[:28]
	blob := &models.ActiveStorageBlob{Key: key, Filename: filename, ContentType: "image/jpeg", ServiceName: "local", ByteSize: 3}
	require.NoError(t, app.DB.Create(blob).Error)
	att := &models.ActiveStorageAttachment{Name: "images", RecordType: "RvListing", RecordID: listingID, BlobID: blob.ID}
	require.NoError(t, app.DB.Create(att).Error)
	return blob, att
}
