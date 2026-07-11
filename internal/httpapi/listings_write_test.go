//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// --- helpers ---------------------------------------------------------------

func seedOwner(t *testing.T, app *httpapi.App, tag string) *models.User {
	t.Helper()
	return testutil.SeedUser(t, app, fmt.Sprintf("%s-%d@example.com", tag, testutil.UniqueID()), "Password123!")
}

// buildListingMultipart builds a multipart/form-data body with listing[field]
// values and one listing[images][] file (image/png), as Rails' create spec sends.
func buildListingMultipart(t *testing.T, values map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range values {
		require.NoError(t, w.WriteField(k, v))
	}
	if filename != "" {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, "listing[images][]", filename))
		h.Set("Content-Type", "image/png")
		part, err := w.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func doAuthJSON(t *testing.T, method, url, auth, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// --- GET /api/v1/listings/mine --------------------------------------------

func TestListingsMine_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/api/v1/listings/mine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListingsMine_ReturnsOnlyOwnersListings(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mine-owner")
	other := seedOwner(t, app, "mine-other")
	mine := seedListing(t, app, owner.ID, "100")
	theirs := seedListing(t, app, other.ID, "100")

	resp := doAuthJSON(t, http.MethodGet, server.URL+"/api/v1/listings/mine", testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listings []struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listings))
	ids := map[int64]bool{}
	for _, l := range listings {
		ids[l.ID] = true
	}
	require.True(t, ids[mine.ID], "own listing present")
	require.False(t, ids[theirs.ID], "other owner's listing absent")
}

func TestListingsMine_EmptyArray(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "mine-empty")
	resp := doAuthJSON(t, http.MethodGet, server.URL+"/api/v1/listings/mine", testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "[]", string(body))
}

// --- POST /api/v1/listings -------------------------------------------------

func TestCreateListing_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	body, ct := buildListingMultipart(t, map[string]string{"listing[title]": "X"}, "test.png", []byte("png"))
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/listings", body)
	req.Header.Set("Content-Type", ct)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateListing_ValidMultipart_PersistsWithRegionAndImage(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "create-owner")
	body, ct := buildListingMultipart(t, map[string]string{
		"listing[title]":         "Cozy Caravan",
		"listing[description]":   "A lovely caravan",
		"listing[rv_type]":       "caravan",
		"listing[town]":          "Byron Bay",
		"listing[state]":         "NSW",
		"listing[postcode]":      "2481",
		"listing[price_per_day]": "150",
		"listing[max_guests]":    "4",
	}, "test.png", []byte("pngdata"))

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/listings", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", testutil.AuthHeader(t, app, owner))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		RvType   string `json:"rv_type"`
		Town     string `json:"town"`
		State    string `json:"state"`
		Postcode string `json:"postcode"`
		OwnerID  int64  `json:"owner_id"`
		Images   []struct {
			ID  int64  `json:"id"`
			URL string `json:"url"`
		} `json:"images"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "Cozy Caravan", got.Title)
	require.Equal(t, "caravan", got.RvType)
	require.Equal(t, "Byron Bay", got.Town)
	require.Equal(t, "NSW", got.State)
	require.Equal(t, "2481", got.Postcode)
	require.Equal(t, owner.ID, got.OwnerID)
	require.Len(t, got.Images, 1, "uploaded image attached")
	require.NotZero(t, got.Images[0].ID)

	// Region is resolved canonically on save (ADR-0013) though it is not
	// serialized — assert it on the persisted row.
	var saved models.RvListing
	require.NoError(t, app.DB.First(&saved, got.ID).Error)
	require.NotNil(t, saved.Region)
	require.Equal(t, "byron-bay", *saved.Region)
}

func TestCreateListing_MissingFields_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "create-invalid")
	resp := doAuthJSON(t, http.MethodPost, server.URL+"/api/v1/listings",
		testutil.AuthHeader(t, app, owner), `{"listing":{"title":"X"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var got struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	// Byte-for-byte the message set real Rails returns for new(title:"X"):
	// rv_type/max_guests are NOT NULL with DB defaults, so they are never blank.
	require.Equal(t, []string{
		"Description can't be blank",
		"Town can't be blank",
		"State can't be blank",
		"Postcode can't be blank",
		"Price per day can't be blank",
		"Images must have at least one photo",
	}, got.Errors)
}

// --- PUT /api/v1/listings/:id ---------------------------------------------

func TestUpdateListing_NonOwner_Returns403(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "upd-owner")
	other := seedOwner(t, app, "upd-other")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID),
		testutil.AuthHeader(t, app, other), `{"listing":{"title":"Hax"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateListing_Owner_UpdatesTitle(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "upd-ok")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID),
		testutil.AuthHeader(t, app, owner), `{"listing":{"title":"Changed"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Title string `json:"title"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "Changed", got.Title)
}

func TestUpdateListing_PreservesImages(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "upd-img")
	listing := seedListing(t, app, owner.ID, "100")
	seedListingImage(t, app, listing.ID, "keep.jpg")

	resp := doAuthJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID),
		testutil.AuthHeader(t, app, owner), `{"listing":{"title":"New Title"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Images []struct {
			ID int64 `json:"id"`
		} `json:"images"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Images, 1, "updating other fields must not remove images")
}

func TestUpdateListing_ReresolvesRegion(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "upd-region")
	listing := seedListing(t, app, owner.ID, "100") // Town "Byron Bay"

	resp := doAuthJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID),
		testutil.AuthHeader(t, app, owner), `{"listing":{"town":"Katoomba"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var saved models.RvListing
	require.NoError(t, app.DB.First(&saved, listing.ID).Error)
	require.NotNil(t, saved.Region)
	require.Equal(t, "blue-mountains", *saved.Region, "region re-resolved from updated town")
}

// --- DELETE /api/v1/listings/:id ------------------------------------------

func TestDestroyListing_NonOwner_Returns403(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "del-owner")
	other := seedOwner(t, app, "del-other")
	listing := seedListing(t, app, owner.ID, "100")

	resp := doAuthJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID),
		testutil.AuthHeader(t, app, other), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestDestroyListing_Owner_204AndGone(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "del-ok")
	listing := seedListing(t, app, owner.ID, "100")
	seedListingImage(t, app, listing.ID, "gone.jpg")

	resp := doAuthJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/listings/%d", server.URL, listing.ID),
		testutil.AuthHeader(t, app, owner), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	var count int64
	require.NoError(t, app.DB.Model(&models.RvListing{}).Where("id = ?", listing.ID).Count(&count).Error)
	require.Zero(t, count, "listing deleted")
}
