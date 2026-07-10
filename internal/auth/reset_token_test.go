package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Known-answer test computed independently (Python hashlib/hmac) to lock the
// exact Devise algorithm: HMAC-SHA256 keyed by
// PBKDF2-HMAC-SHA1(secret, "Devise reset_password_token", 65536, 64).
func TestResetTokenDigest_MatchesDeviseKnownAnswer(t *testing.T) {
	const (
		secret = "test-secret-key-base-shared-with-rails"
		raw    = "abc123def456"
		want   = "fe09fd1cd6481e7b292c965e8f8562d121e9fd2b3aa9e9179791fdc445b4807e"
	)
	got, err := ResetTokenDigest(secret, raw)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestResetTokenDigest_Deterministic(t *testing.T) {
	a, err := ResetTokenDigest("s", "raw")
	require.NoError(t, err)
	b, err := ResetTokenDigest("s", "raw")
	require.NoError(t, err)
	require.Equal(t, a, b)
	other, err := ResetTokenDigest("s", "different")
	require.NoError(t, err)
	require.NotEqual(t, a, other)
}

func TestGenerateResetToken_UniqueAndNonEmpty(t *testing.T) {
	a, err := GenerateResetToken()
	require.NoError(t, err)
	require.NotEmpty(t, a)
	b, err := GenerateResetToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}
