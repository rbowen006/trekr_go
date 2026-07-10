//go:build integration

package httpapi_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/storage"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// noRedirectClient stops the client from following the 302 so we can inspect it.
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func seedBlob(t *testing.T, app *httpapi.App, contentType string) *models.ActiveStorageBlob {
	t.Helper()
	key := fmt.Sprintf("k%027d", testutil.UniqueID())[:28]
	blob := &models.ActiveStorageBlob{
		Key:         key,
		Filename:    "photo.jpg",
		ContentType: contentType,
		ServiceName: "local",
		ByteSize:    3,
	}
	require.NoError(t, app.DB.Create(blob).Error)
	return blob
}

func TestBlobRedirect_ValidSignedID_RedirectsToDiskURL(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	blob := seedBlob(t, app, "image/jpeg")
	signedID, err := storage.GenerateSignedID(app.Config.SecretKeyBase, blob.ID)
	require.NoError(t, err)

	resp, err := noRedirectClient().Get(server.URL + "/rails/active_storage/blobs/redirect/" + signedID + "/photo.jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, "/rails/active_storage/disk/"), "got %q", loc)

	// The disk URL's key must decode back to this blob's storage key.
	enc := strings.TrimPrefix(loc, "/rails/active_storage/disk/")
	enc = enc[:strings.LastIndex(enc, "/")]
	data, err := storage.VerifyBlobKey(app.Config.SecretKeyBase, enc)
	require.NoError(t, err)
	require.Equal(t, blob.Key, data.Key)
	require.Equal(t, "image/jpeg", data.ContentType)
}

func TestBlobRedirect_InvalidSignedID_Returns404(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp, err := noRedirectClient().Get(server.URL + "/rails/active_storage/blobs/redirect/bogus--deadbeef/photo.jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDiskShow_ValidKey_ServesFileBytes(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	blob := seedBlob(t, app, "image/jpeg")
	content := []byte("JPG")
	writeBlobFile(t, app.Config.StorageRoot, blob.Key, content)

	encodedKey, err := storage.GenerateBlobKey(app.Config.SecretKeyBase, storage.BlobKeyData{
		Key:         blob.Key,
		Disposition: storage.ContentDisposition("inline", blob.Filename),
		ContentType: blob.ContentType,
		ServiceName: "local",
	}, storage.ServiceURLExpiry)
	require.NoError(t, err)

	resp, err := http.Get(server.URL + "/rails/active_storage/disk/" + encodedKey + "/photo.jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Content-Disposition"), "photo.jpg")
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, content, body)
}

func TestDiskShow_InvalidKey_Returns404(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/rails/active_storage/disk/bogus--deadbeef/photo.jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestBlobRedirectThenDisk_EndToEnd(t *testing.T) {
	app := testutil.NewTestApp(t)
	app.Config.StorageRoot = t.TempDir()
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	blob := seedBlob(t, app, "image/png")
	content := []byte("PNGDATA")
	writeBlobFile(t, app.Config.StorageRoot, blob.Key, content)

	signedID, err := storage.GenerateSignedID(app.Config.SecretKeyBase, blob.ID)
	require.NoError(t, err)

	// Follow the redirect all the way to the served bytes.
	resp, err := http.Get(server.URL + "/rails/active_storage/blobs/redirect/" + signedID + "/photo.jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, content, body)
}

func writeBlobFile(t *testing.T, root, key string, content []byte) {
	t.Helper()
	dir := filepath.Join(root, key[0:2], key[2:4])
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, key), content, 0o644))
}
