package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Devise's token generator derives its HMAC key with
// ActiveSupport::KeyGenerator defaults: PBKDF2-HMAC-SHA1, 2**16 iterations,
// 64-byte key, salted with "Devise <column>".
const (
	resetTokenSalt       = "Devise reset_password_token"
	resetTokenIterations = 65536 // 2**16
	resetTokenKeyLen     = 64
)

// GenerateResetToken returns a random URL-safe raw reset token — the value that
// would be emailed to the user. Only its digest is persisted. The format is not
// compat-sensitive; only the digest computation must match Devise.
func GenerateResetToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate reset token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ResetTokenDigest computes the value stored in users.reset_password_token for a
// raw token, matching Devise::TokenGenerator: HMAC-SHA256 of the raw token keyed
// by PBKDF2-HMAC-SHA1(secret, "Devise reset_password_token", 65536, 64).
func ResetTokenDigest(secret, raw string) (string, error) {
	key, err := pbkdf2.Key(sha1.New, secret, []byte(resetTokenSalt), resetTokenIterations, resetTokenKeyLen)
	if err != nil {
		return "", fmt.Errorf("derive reset token key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
