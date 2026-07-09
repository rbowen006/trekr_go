//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestRegisterUser_PersistsUserWithBcryptHash(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	email := fmt.Sprintf("register-%d@example.com", testutil.UniqueID())
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

	var parsed struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.NotZero(t, parsed.User.ID)

	var user models.User
	require.NoError(t, app.DB.First(&user, parsed.User.ID).Error)
	require.Equal(t, email, user.Email)
	require.Equal(t, "Alice", user.Name)
	require.NotEmpty(t, user.EncryptedPassword)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.EncryptedPassword), []byte("Password123!")))
}
