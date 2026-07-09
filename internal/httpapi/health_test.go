package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func TestGetUp_Returns200(t *testing.T) {
	server := testutil.NewTestServer(t, testutil.NewApp(testutil.DefaultConfig()))
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/up")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
