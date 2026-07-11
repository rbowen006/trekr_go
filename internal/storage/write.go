package storage

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
)

// keyLength is ActiveStorage::Blob::MINIMUM_TOKEN_LENGTH — blob keys are 28-char
// base36 tokens (SecureRandom.base36(28)).
const keyLength = 28

const base36 = "0123456789abcdefghijklmnopqrstuvwxyz"

// GenerateKey returns a random 28-char base36 blob key, mirroring
// ActiveStorage::Blob.generate_unique_secure_token. Uniqueness is enforced by
// the unique index on active_storage_blobs.key; 28 base36 chars make a
// collision astronomically unlikely.
func GenerateKey() (string, error) {
	b := make([]byte, keyLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = base36[int(b[i])%len(base36)]
	}
	return string(b), nil
}

// Checksum returns the base64-encoded MD5 digest Rails stores on a blob
// (Digest::MD5.base64digest of the file bytes).
func Checksum(data []byte) string {
	sum := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// WriteBlob writes data to the disk-service location for key, creating the
// nested <key[0:2]>/<key[2:4]> directories. Mirrors
// ActiveStorage::Service::DiskService#upload. The key is validated by DiskPath
// so it cannot escape the storage root.
func WriteBlob(root, key string, data []byte) error {
	path, err := DiskPath(root, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
