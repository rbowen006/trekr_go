//go:build integration

package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// Mirrors Rails auth_failure_spec: an unauthenticated request to a protected
// /api/v1 endpoint returns Devise's JSend fail shape, not the bare {error: ...}.
func TestProtectedApiRoute_Unauthenticated_ReturnsJSend401(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp, err := http.Post(
		server.URL+"/api/v1/listings/generate_description",
		"application/json",
		strings.NewReader(`{"rv_type":"motorhome"}`),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "fail", body["status"])
	require.NotEmpty(t, body["message"])
	// Must not be Devise's default bare {error: ...} shape.
	require.NotContains(t, body, "error")
}

// A valid token passes the /api/v1 auth gate: an unmapped authed path falls
// through to the group's 404 catch-all — the point is that it is NOT rejected
// with 401. Uses a deliberately unmapped path so it stays a pure auth-gate probe
// as concrete endpoints get implemented.
func TestProtectedApiRoute_Authenticated_PassesAuthGate(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := testutil.SeedUser(t, app, fmt.Sprintf("apigate-%d@example.com", testutil.UniqueID()), "Password123!")

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/does_not_exist", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", testutil.AuthHeader(t, app, user))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
}
