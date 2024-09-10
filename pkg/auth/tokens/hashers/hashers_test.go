package hashers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHasherForHash(t *testing.T) {
	const testSecret = "testsecret"
	scryptHash, err := ScryptHasher{}.CreateHash(testSecret)
	assert.NoError(t, err, "error when creating scrypt hash")
	sha256Hash, err := Sha256Hasher{}.CreateHash(testSecret)
	assert.NoError(t, err, "error when creating sha256 hash")
	sha3Hash, err := Sha3Hasher{}.CreateHash(testSecret)
	assert.NoError(t, err, "error when creating sha3 hash")

	tests := []struct {
		name       string
		hash       string
		wantHasher Hasher
		wantErr    bool
	}{
		{
			name:       "scrypt hash",
			hash:       scryptHash,
			wantHasher: ScryptHasher{},
			wantErr:    false,
		},
		{
			name:       "sha256 hash",
			hash:       sha256Hash,
			wantHasher: Sha256Hasher{},
			wantErr:    false,
		},
		{
			name:       "sha3 hash",
			hash:       sha3Hash,
			wantHasher: Sha3Hasher{},
			wantErr:    false,
		},
		{
			name:       "invalid hash",
			hash:       "thisisnotahash",
			wantHasher: nil,
			wantErr:    true,
		},
		{
			name:       "invalid hash version",
			hash:       "$4:some-salt-here:some-secret-here",
			wantHasher: nil,
			wantErr:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			hasher, err := GetHasherForHash(test.hash)
			if test.wantErr {
				assert.Error(t, err, "wanted error but did not get one")
			} else {
				assert.NoError(t, err, "got an error but did not get one")
				assert.IsTypef(t, hasher, test.wantHasher, "did not get the expected hasher")
			}
		})
	}
}

func TestGetHasher(t *testing.T) {
	t.Parallel()
	assert.IsTypef(t, Sha3Hasher{}, GetHasher(), "expected SHA3 to be the default hasher")
}

func TestGetHashVersion(t *testing.T) {
	tests := []struct {
		name            string
		hash            string
		wantHashVersion HashVersion
		wantErr         bool
	}{
		{
			name:            "test valid hash",
			hash:            "$1:some-salt-here:some-secret-here",
			wantHashVersion: ScryptVersion,
			wantErr:         false,
		},
		{
			name:    "test bad hash format",
			hash:    "$1:some-secret-here",
			wantErr: true,
		},
		{
			name:    "test bad hash version",
			hash:    "$not-a-number:some-salt-here:some-secret-here",
			wantErr: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			hashVersion, err := GetHashVersion(test.hash)
			if test.wantErr {
				assert.Error(t, err, "Wanted error but did not get one")
			} else {
				assert.NoError(t, err, "Wanted error but did not get one")
				assert.Equal(t, test.wantHashVersion, hashVersion, "did not get expected hash version")
			}
		})
	}
}
