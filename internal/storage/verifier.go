// Package storage reproduces the Active Storage read path: Rails-compatible
// signed IDs / verified keys and the on-disk blob layout.
package storage

import (
	"bytes"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Active Storage signs with app.message_verifier("ActiveStorage"): the secret is
// derived from secret_key_base with PBKDF2-HMAC-SHA256 (Rails' KeyGenerator
// default), while the message HMAC itself uses SHA1.
const (
	verifierSalt       = "ActiveStorage"
	verifierIterations = 1000
	verifierKeyLen     = 64
	signedIDPurpose    = "blob_id"
	blobKeyPurpose     = "blob_key"
)

// railsMessage is the ActiveSupport::MessageVerifier metadata envelope. Field
// order (data, exp, pur) matches Rails so generated tokens are byte-identical.
type railsMessage struct {
	Rails railsMeta `json:"_rails"`
}

type railsMeta struct {
	Data json.RawMessage `json:"data"`
	Exp  string          `json:"exp,omitempty"`
	Pur  string          `json:"pur"`
}

// BlobKeyData is the payload signed into a disk-service key.
type BlobKeyData struct {
	Key         string `json:"key"`
	Disposition string `json:"disposition"`
	ContentType string `json:"content_type"`
	ServiceName string `json:"service_name"`
}

func verifierSecret(secretKeyBase string) ([]byte, error) {
	return pbkdf2.Key(sha256.New, secretKeyBase, []byte(verifierSalt), verifierIterations, verifierKeyLen)
}

// generate produces a MessageVerifier token: base64(json envelope)--hex(HMAC-SHA1).
func generate(secretKeyBase string, data json.RawMessage, purpose, exp string) (string, error) {
	secret, err := verifierSecret(secretKeyBase)
	if err != nil {
		return "", err
	}
	msg := railsMessage{Rails: railsMeta{Data: data, Exp: exp, Pur: purpose}}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // Ruby's JSON does not escape <, >, &
	if err := enc.Encode(msg); err != nil {
		return "", fmt.Errorf("encode message: %w", err)
	}
	payload := base64.StdEncoding.EncodeToString(bytes.TrimRight(buf.Bytes(), "\n"))

	mac := hmac.New(sha1.New, secret)
	mac.Write([]byte(payload))
	sig := fmt.Sprintf("%x", mac.Sum(nil))
	return payload + "--" + sig, nil
}

// verify checks the signature, purpose, and expiry and returns the data payload.
func verify(secretKeyBase, token, purpose string) (json.RawMessage, error) {
	secret, err := verifierSecret(secretKeyBase)
	if err != nil {
		return nil, err
	}
	payload, sig, ok := strings.Cut(token, "--")
	if !ok {
		return nil, fmt.Errorf("invalid token: missing signature")
	}

	mac := hmac.New(sha1.New, secret)
	mac.Write([]byte(payload))
	expected := fmt.Sprintf("%x", mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) != 1 {
		return nil, fmt.Errorf("invalid token: signature mismatch")
	}

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid token: bad base64: %w", err)
	}
	var msg railsMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("invalid token: bad json: %w", err)
	}
	if msg.Rails.Pur != purpose {
		return nil, fmt.Errorf("invalid token: purpose mismatch")
	}
	if msg.Rails.Exp != "" {
		exp, err := time.Parse(time.RFC3339, msg.Rails.Exp)
		if err != nil {
			return nil, fmt.Errorf("invalid token: bad expiry: %w", err)
		}
		if time.Now().After(exp) {
			return nil, fmt.Errorf("invalid token: expired")
		}
	}
	return msg.Rails.Data, nil
}

// GenerateSignedID returns the permanent signed ID for a blob (purpose blob_id),
// as embedded in /rails/active_storage/blobs/redirect/<signed_id>/<filename>.
func GenerateSignedID(secretKeyBase string, blobID int64) (string, error) {
	return generate(secretKeyBase, json.RawMessage(strconv.FormatInt(blobID, 10)), signedIDPurpose, "")
}

// VerifySignedID decodes a signed blob ID.
func VerifySignedID(secretKeyBase, token string) (int64, error) {
	data, err := verify(secretKeyBase, token, signedIDPurpose)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid signed id payload: %w", err)
	}
	return id, nil
}

// GenerateBlobKey returns the expiring disk-service key (purpose blob_key) used
// in /rails/active_storage/disk/<encoded_key>/<filename>.
func GenerateBlobKey(secretKeyBase string, data BlobKeyData, expiresIn time.Duration) (string, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal blob key: %w", err)
	}
	exp := time.Now().UTC().Add(expiresIn).Format("2006-01-02T15:04:05.000") + "Z"
	return generate(secretKeyBase, payload, blobKeyPurpose, exp)
}

// VerifyBlobKey decodes a disk-service key.
func VerifyBlobKey(secretKeyBase, token string) (BlobKeyData, error) {
	data, err := verify(secretKeyBase, token, blobKeyPurpose)
	if err != nil {
		return BlobKeyData{}, err
	}
	var out BlobKeyData
	if err := json.Unmarshal(data, &out); err != nil {
		return BlobKeyData{}, fmt.Errorf("invalid blob key payload: %w", err)
	}
	return out, nil
}
