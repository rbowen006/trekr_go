package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/internal/config"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func TestCORSPreflight_ReturnsAllowedOrigin(t *testing.T) {
	server := testutil.NewTestServer(t, config.Config{
		AllowedOrigins: "http://localhost:5173",
	})
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodOptions, server.URL+"/api/v1/listings", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "http://localhost:5173", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
}

func TestCORSResponse_ExposesAuthorizationHeader(t *testing.T) {
	server := testutil.NewTestServer(t, testutil.DefaultConfig())
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/up", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:5173")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "http://localhost:5173", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Authorization", resp.Header.Get("Access-Control-Expose-Headers"))
}
