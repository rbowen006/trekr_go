package testutil

import (
	"net/http/httptest"
	"testing"

	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/httpapi"
)

// NewTestServer returns an httptest server wired to the real application router.
func NewTestServer(t *testing.T, cfg config.Config) *httptest.Server {
	t.Helper()
	return httptest.NewServer(httpapi.NewRouter(cfg))
}

// DefaultConfig returns config suitable for most HTTP tests.
func DefaultConfig() config.Config {
	return config.Config{
		Port:           "3000",
		AllowedOrigins: "http://localhost:5173",
	}
}
