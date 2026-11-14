package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DiskCAS_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	cas, err := NewDiskCAS(dir)
	require.NoError(t, err)

	content := []byte("test chart archive tarball content")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	assert.False(t, cas.Has(digest))

	_, err = cas.Get(digest)
	assert.ErrorIs(t, err, ErrNotFound)

	// Put with matching digest (and sha256: prefix)
	err = cas.Put("sha256:"+digest, bytes.NewReader(content))
	require.NoError(t, err)

	// Has should now be true with or without prefix
	assert.True(t, cas.Has(digest))
	assert.True(t, cas.Has("sha256:"+digest))

	// Get should now succeed
	rc, err := cas.Get(digest)
	require.NoError(t, err)
	defer rc.Close()

	readBytes, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, readBytes)
}

func Test_DiskCAS_PutWithoutDigest(t *testing.T) {
	dir := t.TempDir()
	cas, err := NewDiskCAS(dir)
	require.NoError(t, err)

	content := []byte("anonymous chart content")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	err = cas.Put("", bytes.NewReader(content))
	require.NoError(t, err)

	assert.True(t, cas.Has(digest))

	rc, err := cas.Get(digest)
	require.NoError(t, err)
	defer rc.Close()

	readBytes, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, readBytes)
}

func Test_DiskCAS_DigestMismatch(t *testing.T) {
	dir := t.TempDir()
	cas, err := NewDiskCAS(dir)
	require.NoError(t, err)

	content := []byte("corrupted chart content")
	badDigest := strings.Repeat("a", 64)

	err = cas.Put(badDigest, bytes.NewReader(content))
	assert.ErrorIs(t, err, ErrDigestMismatch)
	assert.False(t, cas.Has(badDigest))
}

func Test_DiskCAS_GetCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	cas, err := NewDiskCAS(dir)
	require.NoError(t, err)

	content := []byte("original chart archive content")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	err = cas.Put(digest, bytes.NewReader(content))
	require.NoError(t, err)

	// Tamper with the file on disk
	target, err := cas.Path(digest)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(target, 0o600))
	require.NoError(t, os.WriteFile(target, []byte("tampered content"), 0o600))

	// Get should detect the corruption, evict the file, and return ErrDigestMismatch / ErrNotFound
	_, err = cas.Get(digest)
	assert.ErrorIs(t, err, ErrDigestMismatch)
	assert.ErrorIs(t, err, ErrNotFound)

	// Corrupted file must be removed from cache
	assert.False(t, cas.Has(digest))
}

func Test_NormalizeDigest(t *testing.T) {
	valid := strings.Repeat("f", 64)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "standard hex", input: valid, want: valid},
		{name: "with sha256 prefix", input: "sha256:" + valid, want: valid},
		{name: "uppercase hex", input: strings.ToUpper(valid), want: valid},
		{name: "with whitespace", input: "  sha256:" + valid + "  ", want: valid},
		{name: "too short", input: "abc", wantErr: true},
		{name: "too long", input: valid + "a", wantErr: true},
		{name: "path traversal characters", input: "../../etc/passwd" + strings.Repeat("0", 47), wantErr: true},
		{name: "non-hex character", input: strings.Repeat("0", 63) + "g", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDigest(tt.input)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidDigest)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
