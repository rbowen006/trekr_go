package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKey_ShapeAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		key, err := GenerateKey()
		require.NoError(t, err)
		require.Len(t, key, 28, "ActiveStorage keys are 28-char base36 tokens")
		for _, c := range key {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z'),
				"key %q has non-base36 char %q", key, c)
		}
		assert.False(t, seen[key], "keys must be unique")
		seen[key] = true
	}
}

// Checksum matches Rails' Digest::MD5.base64digest, stored on active_storage_blobs.
func TestChecksum_MatchesRailsBase64MD5(t *testing.T) {
	assert.Equal(t, "1B2M2Y8AsgTpgAmY7PhCfg==", Checksum([]byte("")))
	assert.Equal(t, "kAFQmDzST7DWlj99KOF/cg==", Checksum([]byte("abc")))
}

// WriteBlob lays the file out like DiskService#upload: <root>/<k0:2>/<k2:4>/<key>.
func TestWriteBlob_DiskLayout(t *testing.T) {
	root := t.TempDir()
	key := "abcd1234efgh5678ijkl9012mnop"
	content := []byte("file bytes")

	require.NoError(t, WriteBlob(root, key, content))

	want := filepath.Join(root, "ab", "cd", key)
	got, err := os.ReadFile(want)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestWriteBlob_RejectsBadKey(t *testing.T) {
	require.Error(t, WriteBlob(t.TempDir(), "../escape", []byte("x")))
}
