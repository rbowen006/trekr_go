//go:build integration

package httpapi_test

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/rbowen/trekr_go/internal/models"
	"github.com/rbowen/trekr_go/test/testutil"
	"github.com/stretchr/testify/require"
)

// Devise downcases email on save (case_insensitive_keys). Registration must do
// the same, otherwise a mixed-case registration can never sign in because the
// sign-in lookup is downcased.
func TestRegisterThenSignIn_MixedCaseEmail_Succeeds(t *testing.T) {
	app := testutil.NewTestApp(t)
	server := testutil.NewTestServer(t, app)
	t.Cleanup(server.Close)

	id := testutil.UniqueID()
	mixed := fmt.Sprintf("MixedCase-%d@Example.com", id)
	lower := fmt.Sprintf("mixedcase-%d@example.com", id)

	regBody := []byte(fmt.Sprintf(
		`{"user":{"email":%q,"password":"Password123!","password_confirmation":"Password123!","name":"Alice"}}`,
		mixed,
	))
	reg, err := http.Post(server.URL+"/users", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	t.Cleanup(func() { _ = reg.Body.Close() })
	require.Equal(t, http.StatusCreated, reg.StatusCode)

	// The persisted email must be normalized to lowercase.
	var user models.User
	require.NoError(t, app.DB.Where("email = ?", lower).First(&user).Error)
	require.Equal(t, lower, user.Email)

	// Sign-in succeeds regardless of the casing supplied.
	signIn, err := http.Post(server.URL+"/users/sign_in", "application/json", bytes.NewReader(signInBody(mixed, "Password123!")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = signIn.Body.Close() })
	require.Equal(t, http.StatusOK, signIn.StatusCode)
	require.True(t, len(signIn.Header.Get("Authorization")) > 0)
}
