//go:build integration

package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func TestSignOut_ValidToken_Returns204(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := testutil.SeedUser(t, app, fmt.Sprintf("signout-%d@example.com", testutil.UniqueID()), "Password123!")

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/users/sign_out", nil)
	req.Header.Set("Authorization", testutil.AuthHeader(t, app, user))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// Devise's Null revocation strategy makes sign-out a no-op that never 401s,
// even when the token is unusable.
func TestSignOut_MalformedToken_Returns204(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/users/sign_out", nil)
	req.Header.Set("Authorization", "Bearer garbage.invalid.token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}
