package auth

import (
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testSecret = "shared-secret-key-base"

func TestIssueToken_RoundTripsThroughParseToken(t *testing.T) {
	token, err := IssueToken(testSecret, 42)
	require.NoError(t, err)

	userID, err := ParseToken(testSecret, token)
	require.NoError(t, err)
	require.Equal(t, int64(42), userID)
}

func TestIssueToken_ProducesDeviseClaimShape(t *testing.T) {
	token, err := IssueToken(testSecret, 7)
	require.NoError(t, err)

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	require.NoError(t, err)
	require.Equal(t, "7", claims["sub"])
	require.Equal(t, "user", claims["scp"])
	require.Contains(t, claims, "iat")
	require.Contains(t, claims, "exp")
	require.Contains(t, claims, "jti")
}

func TestParseToken_WrongSecret_Rejected(t *testing.T) {
	token, err := IssueToken(testSecret, 1)
	require.NoError(t, err)

	_, err = ParseToken("different-secret", token)
	require.Error(t, err)
}

func TestParseToken_RejectsNoneAlgorithm(t *testing.T) {
	// An unsigned "alg: none" token must not be accepted.
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": "1", "scp": "user"}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ParseToken(testSecret, unsigned)
	require.Error(t, err)
}

func TestParseToken_MalformedToken_Rejected(t *testing.T) {
	_, err := ParseToken(testSecret, "not.a.jwt")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "parse jwt"))
}
