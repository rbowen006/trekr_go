package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyPassword_RailsDeviseHash(t *testing.T) {
	// Hash produced by Devise BCrypt for password "password" at cost 12.
	const railsHash = "$2a$12$Mj5XBfMRMuggcBtcac4lVej4IH8NVNvp.FBG6NafvtOaJjIcUlnLG"

	require.True(t, VerifyPassword("password", railsHash))
	require.False(t, VerifyPassword("wrong", railsHash))
}

func TestHashPassword_UsesDeviseCost(t *testing.T) {
	hash, err := HashPassword("Password123!")
	require.NoError(t, err)
	require.Contains(t, hash, "$2a$12$")
	require.True(t, VerifyPassword("Password123!", hash))
}
