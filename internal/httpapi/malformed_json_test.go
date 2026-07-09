package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func TestMalformedJSON_ApiEndpoint_ReturnsJSendFail(t *testing.T) {
	server := testutil.NewTestServer(t, testutil.NewApp(testutil.DefaultConfig()))
	t.Cleanup(server.Close)

	resp := postMalformedJSON(t, server.URL+"/api/v1/listings/generate_description", `{"rv_type":"motorhome", BROKEN`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assertJSendFail(t, resp)
}

func TestMalformedJSON_DeviseEndpoint_ReturnsJSendFail(t *testing.T) {
	server := testutil.NewTestServer(t, testutil.NewApp(testutil.DefaultConfig()))
	t.Cleanup(server.Close)

	resp := postMalformedJSON(t, server.URL+"/users/sign_in", `{"user":{"email": BROKEN`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assertJSendFail(t, resp)
}

func postMalformedJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()

	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	return resp
}

func assertJSendFail(t *testing.T, resp *http.Response) {
	t.Helper()
	t.Cleanup(func() { _ = resp.Body.Close() })

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body map[string]string
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "fail", body["status"])
	require.Equal(t, "Malformed request body", body["message"])
}
