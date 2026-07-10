package storage

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// ServiceURLExpiry matches ActiveStorage.service_urls_expire_in (5 minutes).
const ServiceURLExpiry = 5 * time.Minute

// DiskPath returns the on-disk location for a blob key using Active Storage's
// disk service layout: <root>/<key[0:2]>/<key[2:4]>/<key>. It rejects keys that
// could escape the storage root.
func DiskPath(root, key string) (string, error) {
	if len(key) < 4 || strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid blob key")
	}
	return filepath.Join(root, key[0:2], key[2:4], key), nil
}

// ContentDisposition formats a Content-Disposition header value the way
// ActionDispatch::Http::ContentDisposition does.
func ContentDisposition(disposition, filename string) string {
	if disposition != "attachment" {
		disposition = "inline"
	}
	return fmt.Sprintf("%s; filename=%q; filename*=UTF-8''%s", disposition, filename, url.PathEscape(filename))
}
