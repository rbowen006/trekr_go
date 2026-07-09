//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	mw "github.com/rbowen/trekr_go/internal/httpapi/middleware"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// railsStyleToken builds a token exactly as warden-jwt_auth would, signed with
// the shared secret — independent of our production issuer, so accepting it
// proves cross-backend (Rails -> Go) parity.
func railsStyleToken(t *testing.T, userID int64) string {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"scp": "user",
		"aud": nil,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
		"jti": "11111111-2222-3333-4444-555555555555",
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testutil.TestSecret))
	require.NoError(t, err)
	return signed
}

func TestProtectedRoute_RailsIssuedToken_Accepted(t *testing.T) {
	app := testutil.NewTestApp(t)

	email := fmt.Sprintf("protected-%d@example.com", testutil.UniqueID())
	user := testutil.SeedUser(t, app, email, "Password123!")

	r := chi.NewRouter()
	r.With(mw.RequireAuth(testutil.TestSecret, app.DB)).Get("/probe", func(w http.ResponseWriter, r *http.Request) {
		u, ok := mw.CurrentUser(r.Context())
		require.True(t, ok)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": u.ID})
	})
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/probe", nil)
	req.Header.Set("Authorization", "Bearer "+railsStyleToken(t, user.ID))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	require.EqualValues(t, user.ID, body["id"])
}

func TestProtectedRoute_NoToken_Returns401(t *testing.T) {
	app := testutil.NewTestApp(t)

	r := chi.NewRouter()
	r.With(mw.RequireAuth(testutil.TestSecret, app.DB)).Get("/probe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/probe")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]string
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "fail", body["status"])
	require.Equal(t, "You need to sign in or sign up before continuing.", body["message"])
}

func TestProtectedRoute_InvalidToken_Returns401(t *testing.T) {
	app := testutil.NewTestApp(t)

	r := chi.NewRouter()
	r.With(mw.RequireAuth(testutil.TestSecret, app.DB)).Get("/probe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/probe", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
