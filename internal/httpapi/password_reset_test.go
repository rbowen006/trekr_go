//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rbowen/trekr_go/internal/auth"
	"github.com/rbowen/trekr_go/internal/httpapi"
	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	return resp
}

func doJSON(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// seedResetToken puts a valid Devise-compatible reset token on the user and
// returns the raw token the client would present.
func seedResetToken(t *testing.T, app *httpapi.App, user *models.User, sentAt time.Time) string {
	t.Helper()
	raw, err := auth.GenerateResetToken()
	require.NoError(t, err)
	digest, err := auth.ResetTokenDigest(app.Config.SecretKeyBase, raw)
	require.NoError(t, err)
	user.ResetPasswordToken = &digest
	user.ResetPasswordSentAt = &sentAt
	require.NoError(t, app.DB.Save(user).Error)
	return raw
}

func TestPasswordReset_Create_UnknownEmail_Returns200NoSideEffect(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	resp := postJSON(t, server.URL+"/users/password", `{"user":{"email":"nobody-`+fmt.Sprint(testutil.UniqueID())+`@example.com"}}`)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPasswordReset_Create_KnownEmail_Returns200AndSetsToken(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("reset-create-%d@example.com", testutil.UniqueID())
	user := testutil.SeedUser(t, app, email, "Password123!")
	require.Nil(t, user.ResetPasswordToken)

	resp := postJSON(t, server.URL+"/users/password", fmt.Sprintf(`{"user":{"email":%q}}`, email))
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var reloaded models.User
	require.NoError(t, app.DB.First(&reloaded, user.ID).Error)
	require.NotNil(t, reloaded.ResetPasswordToken)
	require.NotEmpty(t, *reloaded.ResetPasswordToken)
	require.NotNil(t, reloaded.ResetPasswordSentAt)
}

func TestPasswordReset_Update_ValidToken_Returns200AndChangesPassword(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := testutil.SeedUser(t, app, fmt.Sprintf("reset-ok-%d@example.com", testutil.UniqueID()), "OldPassword1")
	raw := seedResetToken(t, app, user, time.Now().UTC())

	body := fmt.Sprintf(`{"user":{"reset_password_token":%q,"password":"newpassword1","password_confirmation":"newpassword1"}}`, raw)
	resp := doJSON(t, http.MethodPut, server.URL+"/users/password", body)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var reloaded models.User
	require.NoError(t, app.DB.First(&reloaded, user.ID).Error)
	require.True(t, auth.VerifyPassword("newpassword1", reloaded.EncryptedPassword))
	require.Nil(t, reloaded.ResetPasswordToken, "token should be cleared after use")
}

func TestPasswordReset_Update_InvalidToken_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	body := `{"user":{"reset_password_token":"bogus","password":"newpassword1","password_confirmation":"newpassword1"}}`
	resp := doJSON(t, http.MethodPut, server.URL+"/users/password", body)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	requireErrorsPresent(t, resp)
}

func TestPasswordReset_Update_ExpiredToken_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := testutil.SeedUser(t, app, fmt.Sprintf("reset-exp-%d@example.com", testutil.UniqueID()), "OldPassword1")
	raw := seedResetToken(t, app, user, time.Now().UTC().Add(-7*time.Hour)) // > 6h old

	body := fmt.Sprintf(`{"user":{"reset_password_token":%q,"password":"newpassword1","password_confirmation":"newpassword1"}}`, raw)
	resp := doJSON(t, http.MethodPut, server.URL+"/users/password", body)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	requireErrorsPresent(t, resp)
}

func TestPasswordReset_Update_PasswordMismatch_Returns422(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	user := testutil.SeedUser(t, app, fmt.Sprintf("reset-mismatch-%d@example.com", testutil.UniqueID()), "OldPassword1")
	raw := seedResetToken(t, app, user, time.Now().UTC())

	body := fmt.Sprintf(`{"user":{"reset_password_token":%q,"password":"newpassword1","password_confirmation":"different"}}`, raw)
	resp := doJSON(t, http.MethodPut, server.URL+"/users/password", body)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	requireErrorsPresent(t, resp)
}

func requireErrorsPresent(t *testing.T, resp *http.Response) {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.NotEmpty(t, body.Errors)
}
