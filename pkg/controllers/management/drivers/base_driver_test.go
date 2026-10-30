package drivers

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHasher(t *testing.T) {
	tests := []struct {
		name       string
		hash       string
		wantHasher bool
		checkType  func(t *testing.T, d *BaseDriver)
	}{
		{
			name:       "empty hash",
			hash:       "",
			wantHasher: false,
		},
		{
			name:       "invalid length 10",
			hash:       "0123456789",
			wantHasher: false,
		},
		{
			name:       "md5 32 chars",
			hash:       "5eb63bbbe01eeed093cb22bb8f5acdc3", // echo -n "hello world" | md5sum
			wantHasher: true,
			checkType: func(t *testing.T, d *BaseDriver) {
				assert.Equal(t, md5.New(), d.getHasher())
			},
		},
		{
			name:       "sha1 40 chars",
			hash:       "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed", // echo -n "hello world" | sha1sum
			wantHasher: true,
			checkType: func(t *testing.T, d *BaseDriver) {
				assert.Equal(t, sha1.New(), d.getHasher())
			},
		},
		{
			name:       "sha256 64 chars",
			hash:       "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", // echo -n "hello world" | sha256sum
			wantHasher: true,
			checkType: func(t *testing.T, d *BaseDriver) {
				assert.Equal(t, sha256.New(), d.getHasher())
			},
		},
		{
			name:       "sha512 128 chars",
			hash:       "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f", // echo -n "hello world" | sha512sum
			wantHasher: true,
			checkType: func(t *testing.T, d *BaseDriver) {
				assert.Equal(t, sha512.New(), d.getHasher())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &BaseDriver{
				DriverHash: tt.hash,
			}
			hasher := d.getHasher()
			if tt.wantHasher {
				require.NotNil(t, hasher)
				if tt.checkType != nil {
					tt.checkType(t, d)
				}
			} else {
				assert.Nil(t, hasher)
			}
		})
	}
}

func TestStageInvalidHash(t *testing.T) {
	d := &BaseDriver{
		Builtin:    false,
		DriverName: "testdriver",
		DriverHash: "invalid-hash",
	}

	err := d.stage(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hash format: invalid-hash")
}

func TestBaseDriverBinDir(t *testing.T) {
	d := &BaseDriver{DevMode: true}
	assert.Equal(t, "./management-state/bin", d.binDir())

	d.DevMode = false
	assert.Equal(t, "/opt/drivers/management-state/bin", d.binDir())

	d.BinDir = "/custom/bin"
	assert.Equal(t, "/custom/bin", d.binDir())
}

func TestBaseDriverCacheDir(t *testing.T) {
	d := &BaseDriver{
		URL:        "https://example.com/test",
		DriverHash: "123",
		CacheDir:   "/custom/cache",
	}
	assert.True(t, strings.HasPrefix(d.cacheFile(), "/custom/cache/"))
}
