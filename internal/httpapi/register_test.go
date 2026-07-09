//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func TestRegisterUser_Returns201AndUserEnvelope(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("envelope-%d@example.com", testutil.UniqueID())
	body := []byte(fmt.Sprintf(
		`{"user":{"email":%q,"password":"Password123!","password_confirmation":"Password123!","name":"Alice"}}`,
		email,
	))
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
	require.Equal(t, email, user["email"])
	require.Equal(t, "Alice", user["name"])
	require.NotEmpty(t, user["id"])
	require.Nil(t, user["password"])
	require.Nil(t, user["encrypted_password"])
}

func TestRegisterUser_InvalidParams_Returns422ErrorsArray(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
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

func TestRegisterUser_DuplicateEmail_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("duplicate-%d@example.com", testutil.UniqueID())
	payload := []byte(fmt.Sprintf(
		`{"user":{"email":%q,"password":"Password123!","password_confirmation":"Password123!","name":"Alice"}}`,
		email,
	))

	first, err := http.Post(server.URL+"/users", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, first.StatusCode)
	_ = first.Body.Close()

	second, err := http.Post(server.URL+"/users", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	t.Cleanup(func() { _ = second.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, second.StatusCode)

	raw, err := io.ReadAll(second.Body)
	require.NoError(t, err)

	var parsed struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.Contains(t, parsed.Errors, "Email has already been taken")
}
