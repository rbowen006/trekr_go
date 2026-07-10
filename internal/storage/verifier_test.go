package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-base-shared-with-rails"

// Known-answer test: byte-for-byte match of a signed_id computed independently
// (Python) with the exact Active Storage algorithm.
func TestGenerateSignedID_KnownAnswer(t *testing.T) {
	got, err := GenerateSignedID(testSecret, 20)
	require.NoError(t, err)
	require.Equal(t, "eyJfcmFpbHMiOnsiZGF0YSI6MjAsInB1ciI6ImJsb2JfaWQifX0=--664356b8c932571a4087deacda826d6c39252a15", got)
}

func TestSignedID_RoundTrip(t *testing.T) {
	token, err := GenerateSignedID(testSecret, 12345)
	require.NoError(t, err)
	id, err := VerifySignedID(testSecret, token)
	require.NoError(t, err)
	require.Equal(t, int64(12345), id)
}

func TestVerifySignedID_WrongSecret_Rejected(t *testing.T) {
	token, err := GenerateSignedID(testSecret, 1)
	require.NoError(t, err)
	_, err = VerifySignedID("different-secret", token)
	require.Error(t, err)
}

func TestVerifySignedID_Tampered_Rejected(t *testing.T) {
	token, err := GenerateSignedID(testSecret, 1)
	require.NoError(t, err)
	_, err = VerifySignedID(testSecret, token+"x")
	require.Error(t, err)
}

func TestVerifySignedID_WrongPurpose_Rejected(t *testing.T) {
	// A blob_key token must not verify as a signed id.
	token, err := GenerateBlobKey(testSecret, BlobKeyData{Key: "k", ServiceName: "local"}, time.Minute)
	require.NoError(t, err)
	_, err = VerifySignedID(testSecret, token)
	require.Error(t, err)
}

func TestBlobKey_RoundTrip(t *testing.T) {
	data := BlobKeyData{Key: "v29eg9fs6bbmhdp4qt728ptg8ouh", Disposition: `inline; filename="a.jpg"`, ContentType: "image/jpeg", ServiceName: "local"}
	token, err := GenerateBlobKey(testSecret, data, 5*time.Minute)
	require.NoError(t, err)
	got, err := VerifyBlobKey(testSecret, token)
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func TestBlobKey_Expired_Rejected(t *testing.T) {
	token, err := GenerateBlobKey(testSecret, BlobKeyData{Key: "k", ServiceName: "local"}, -1*time.Minute)
	require.NoError(t, err)
	_, err = VerifyBlobKey(testSecret, token)
	require.Error(t, err)
}
