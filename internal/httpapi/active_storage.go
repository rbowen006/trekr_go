package httpapi

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/internal/storage"
)

// blobRedirect turns a permanent signed blob ID into an expiring disk-service
// URL (302), mirroring ActiveStorage::Blobs::RedirectController.
func (app *App) blobRedirect(w http.ResponseWriter, r *http.Request) {
	signedID, filename, ok := splitStoragePath(chi.URLParam(r, "*"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	blobID, err := storage.VerifySignedID(app.Config.SecretKeyBase, signedID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var blob models.ActiveStorageBlob
	if err := app.DB.First(&blob, blobID).Error; err != nil {
		http.NotFound(w, r)
		return
	}

	encodedKey, err := storage.GenerateBlobKey(app.Config.SecretKeyBase, storage.BlobKeyData{
		Key:         blob.Key,
		Disposition: storage.ContentDisposition("inline", blob.Filename),
		ContentType: blob.ContentType,
		ServiceName: blob.ServiceName,
	}, storage.ServiceURLExpiry)
	if err != nil {
		http.Error(w, "could not sign blob url", http.StatusInternalServerError)
		return
	}

	location := "/rails/active_storage/disk/" + encodedKey + "/" + url.PathEscape(filename)
	http.Redirect(w, r, location, http.StatusFound)
}

// diskShow serves a blob's bytes from disk after verifying the expiring key,
// mirroring ActiveStorage::DiskController#show.
func (app *App) diskShow(w http.ResponseWriter, r *http.Request) {
	encodedKey, _, ok := splitStoragePath(chi.URLParam(r, "*"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	data, err := storage.VerifyBlobKey(app.Config.SecretKeyBase, encodedKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	path, err := storage.DiskPath(app.Config.StorageRoot, data.Key)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		http.NotFound(w, r)
		return
	}

	if data.ContentType != "" {
		w.Header().Set("Content-Type", data.ContentType)
	}
	if data.Disposition != "" {
		w.Header().Set("Content-Disposition", data.Disposition)
	}
	http.ServeContent(w, r, data.Key, stat.ModTime(), f)
}

// splitStoragePath separates a "<token>/<filename>" wildcard into its parts,
// splitting on the final slash so a token containing slashes stays intact.
func splitStoragePath(rest string) (token, filename string, ok bool) {
	rest = strings.TrimPrefix(rest, "/")
	i := strings.LastIndex(rest, "/")
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}
