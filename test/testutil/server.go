package testutil

import (
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/db"
	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// NewApp returns an App without a database (for middleware/health tests).
func NewApp(cfg config.Config) *httpapi.App {
	return &httpapi.App{Config: cfg}
}

var uniqueCounter atomic.Int64

// UniqueID returns a monotonically increasing ID for unique test data.
func UniqueID() int64 {
	return uniqueCounter.Add(1)
}

// DefaultConfig returns config suitable for most HTTP tests.
func DefaultConfig() config.Config {
	return config.Config{
		Port:           "3000",
		AllowedOrigins: "http://localhost:5173",
		DatabaseURL:    "postgres://postgres:password@localhost:5433/rv_marketplace_test",
	}
}

// OpenTestDB connects to the test database, skipping when unavailable.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	conn, err := db.Open(DefaultConfig().DatabaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	sqlDB, err := conn.DB()
	require.NoError(t, err)
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })
	return conn
}

// NewTestApp returns an App wired to the test database.
func NewTestApp(t *testing.T) *httpapi.App {
	t.Helper()
	return &httpapi.App{
		Config: DefaultConfig(),
		DB:     OpenTestDB(t),
	}
}

// NewTestServer returns an httptest server wired to the real application router.
func NewTestServer(t *testing.T, app *httpapi.App) *httptest.Server {
	t.Helper()
	if app == nil {
		app = &httpapi.App{Config: DefaultConfig()}
	}
	return httptest.NewServer(httpapi.NewRouter(app))
}
