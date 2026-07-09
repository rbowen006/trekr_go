package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser_Returns201AndUserEnvelope(t *testing.T) {
	server := testutil.NewTestServer(t, testutil.DefaultConfig())
	t.Cleanup(server.Close)

	body := []byte(`{"user":{"email":"a@example.com","password":"Password123!","password_confirmation":"Password123!","name":"Alice"}}`)
	resp, err := http.Post(server.URL+"/users", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))

	user, ok := parsed["user"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "a@example.com", user["email"])
	require.Equal(t, "Alice", user["name"])
	require.NotEmpty(t, user["id"])
	require.Nil(t, user["password"])
	require.Nil(t, user["encrypted_password"])
}

func TestRegisterUser_InvalidParams_Returns422ErrorsArray(t *testing.T) {
	server := testutil.NewTestServer(t, testutil.DefaultConfig())
	t.Cleanup(server.Close)

	body := []byte(`{"user":{"email":"","password":"x","password_confirmation":"y","name":""}}`)
	resp, err := http.Post(server.URL+"/users", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))

	errs, ok := parsed["errors"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, errs)
}

