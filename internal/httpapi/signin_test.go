//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func signInBody(email, password string) []byte {
	return []byte(fmt.Sprintf(`{"user":{"email":%q,"password":%q}}`, email, password))
}

func TestSignIn_ValidCredentials_ReturnsUserJSON(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("signin-%d@example.com", testutil.UniqueID())
	user := testutil.SeedUser(t, app, email, "Password123!")

	resp, err := http.Post(server.URL+"/users/sign_in", "application/json", bytes.NewReader(signInBody(email, "Password123!")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))

	got, ok := parsed["user"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, email, got["email"])
	require.Equal(t, "Seed User", got["name"])
	require.EqualValues(t, user.ID, got["id"])
	require.Nil(t, got["password"])
	require.Nil(t, got["encrypted_password"])
}

func TestSignIn_ValidCredentials_ReturnsAuthorizationHeader(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("signin-jwt-%d@example.com", testutil.UniqueID())
	user := testutil.SeedUser(t, app, email, "Password123!")

	resp, err := http.Post(server.URL+"/users/sign_in", "application/json", bytes.NewReader(signInBody(email, "Password123!")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	authHeader := resp.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(authHeader, "Bearer "), "expected Bearer token, got %q", authHeader)
	raw := strings.TrimPrefix(authHeader, "Bearer ")

	// Decode independently with the shared secret and assert the devise-jwt
	// claim shape a Rails decoder would accept.
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(tok *jwt.Token) (any, error) {
		require.Equal(t, "HS256", tok.Method.Alg())
		return []byte(testutil.TestSecret), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	require.Equal(t, strconv.FormatInt(user.ID, 10), claims["sub"], "sub must be the string user id")
	require.Equal(t, "user", claims["scp"])
	require.Contains(t, claims, "exp")
	require.Contains(t, claims, "iat")
	require.Contains(t, claims, "jti")
}

func assertInvalidCredentials(t *testing.T, resp *http.Response) {
	t.Helper()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Authorization"))
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]string
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "fail", body["status"])
	require.Equal(t, "Invalid Email or password.", body["message"])
}

func TestSignIn_WrongPassword_Returns401(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("signin-wrong-%d@example.com", testutil.UniqueID())
	testutil.SeedUser(t, app, email, "Password123!")

	resp, err := http.Post(server.URL+"/users/sign_in", "application/json", bytes.NewReader(signInBody(email, "WrongPassword!")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	assertInvalidCredentials(t, resp)
}

func TestSignIn_UnknownEmail_Returns401(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("nobody-%d@example.com", testutil.UniqueID())
	resp, err := http.Post(server.URL+"/users/sign_in", "application/json", bytes.NewReader(signInBody(email, "Password123!")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	assertInvalidCredentials(t, resp)
}
