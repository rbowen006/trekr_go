//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// buildImageMultipart builds a multipart body with one top-level images[] file,
// as Rails' ImagesController spec sends (params[:images]).
func buildImageMultipart(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, "images[]", filename))
	h.Set("Content-Type", "image/png")
	part, err := w.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func postImages(t *testing.T, app *httpapi.App, url, auth string) *http.Response {
	t.Helper()
	body, ct := buildImageMultipart(t, "new.png", []byte("pngbytes"))
	req, err := http.NewRequest(http.MethodPost, url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", ct)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// --- POST /api/v1/listings/:listing_id/images -----------------------------

func TestImagesCreate_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "img-auth")
	listing := seedListing(t, app, owner.ID, "100")

	resp := postImages(t, app, fmt.Sprintf("%s/api/v1/listings/%d/images", server.URL, listing.ID), "")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestImagesCreate_ForbidsNonOwner(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "img-owner")
	other := seedOwner(t, app, "img-other")
	listing := seedListing(t, app, owner.ID, "100")

	resp := postImages(t, app, fmt.Sprintf("%s/api/v1/listings/%d/images", server.URL, listing.ID),
		testutil.AuthHeader(t, app, other))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestImagesCreate_AttachesAndReturnsListing(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "img-ok")
	listing := seedListing(t, app, owner.ID, "100")
	seedListingImage(t, app, listing.ID, "existing.jpg")

	resp := postImages(t, app, fmt.Sprintf("%s/api/v1/listings/%d/images", server.URL, listing.ID),
		testutil.AuthHeader(t, app, owner))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got struct {
		Images []struct {
			ID  int64  `json:"id"`
			URL string `json:"url"`
		} `json:"images"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Len(t, got.Images, 2, "existing + newly attached")
	require.NotZero(t, got.Images[0].ID)
	require.NotEmpty(t, got.Images[0].URL)
}

// --- DELETE /api/v1/listings/:listing_id/images/:id -----------------------

func TestImagesDestroy_RequiresAuth(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "imgdel-auth")
	listing := seedListing(t, app, owner.ID, "100")
	_, att := seedListingImage(t, app, listing.ID, "del.jpg")

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/v1/listings/%d/images/%d", server.URL, listing.ID, att.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestImagesDestroy_ForbidsNonOwner(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "imgdel-owner")
	other := seedOwner(t, app, "imgdel-other")
	listing := seedListing(t, app, owner.ID, "100")
	_, att := seedListingImage(t, app, listing.ID, "del.jpg")

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/v1/listings/%d/images/%d", server.URL, listing.ID, att.ID), nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, app, other))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestImagesDestroy_RemovesImageAndReturns204(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	owner := seedOwner(t, app, "imgdel-ok")
	listing := seedListing(t, app, owner.ID, "100")
	_, att := seedListingImage(t, app, listing.ID, "del.jpg")

	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/v1/listings/%d/images/%d", server.URL, listing.ID, att.ID), nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, app, owner))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	var count int64
	require.NoError(t, app.DB.Model(&models.ActiveStorageAttachment{}).Where("id = ?", att.ID).Count(&count).Error)
	require.Zero(t, count, "attachment purged")
}
