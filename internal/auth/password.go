package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DeviseBcryptCost matches Rails Devise default stretches outside test.
const DeviseBcryptCost = 12

// HashPassword returns a Devise-compatible bcrypt hash for storage in encrypted_password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), DeviseBcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a plaintext password against a Devise/Rails bcrypt hash.
func VerifyPassword(password, encryptedPassword string) bool {
	return bcrypt.CompareHashAndPassword([]byte(encryptedPassword), []byte(password)) == nil
}
