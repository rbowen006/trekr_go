package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTExpiry matches warden-jwt_auth's default expiration_time (3600s).
const JWTExpiry = time.Hour

// jwtScope matches the devise warden scope for the :user mapping.
const jwtScope = "user"

// IssueToken returns a devise-jwt-compatible HS256 token for the given user id.
//
// Claims mirror warden-jwt_auth so a Rails devise-jwt decoder accepts the token:
// sub (string user id), scp ("user"), iat, exp, jti.
func IssueToken(secret string, userID int64) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"scp": jwtScope,
		"iat": now.Unix(),
		"exp": now.Add(JWTExpiry).Unix(),
		"jti": uuid.NewString(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// ParseToken verifies a devise-jwt HS256 token with the shared secret and
// returns the user id from the sub claim. It accepts tokens issued by either
// backend (bidirectional parity).
func ParseToken(secret, tokenString string) (int64, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return 0, fmt.Errorf("parse jwt: %w", err)
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return 0, fmt.Errorf("jwt missing sub claim")
	}
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid sub claim %q: %w", sub, err)
	}
	return userID, nil
}
