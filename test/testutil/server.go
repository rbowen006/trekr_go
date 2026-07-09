package testutil

import (
	"net/http/httptest"
	"testing"

	"github.com/rbowen/trekr_go/internal/httpapi"
)

// NewTestServer returns an httptest server wired to the real application router.
func NewTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(httpapi.NewRouter())
}
