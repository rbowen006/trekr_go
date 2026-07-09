package testutil

import (
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbowen/trekr_go/internal/auth"
	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/internal/db"
	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestSecret signs JWTs in tests. In production SECRET_KEY_BASE must equal
// Rails' value; here a fixed value keeps issue/verify self-consistent.
const TestSecret = "test-secret-key-base-shared-with-rails"

// NewApp returns an App without a database (for middleware/health tests).
func NewApp(cfg config.Config) *httpapi.App {
	return &httpapi.App{Config: cfg}
}

// Seed from the wall clock so generated values stay unique across test runs
// against the shared, non-truncated test database.
var uniqueCounter = func() *atomic.Int64 {
	c := &atomic.Int64{}
	c.Store(time.Now().UnixNano())
	return c
}()

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
		SecretKeyBase:  TestSecret,
	}
}

// SeedUser inserts a user with a Devise-compatible bcrypt hash and returns it.
func SeedUser(t *testing.T, app *httpapi.App, email, password string) *models.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	require.NoError(t, err)
	user := &models.User{Email: email, Name: "Seed User", EncryptedPassword: hash}
	require.NoError(t, app.DB.Create(user).Error)
	return user
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
