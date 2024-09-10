// Package hashers provides the various hash methods which can be used to hash tokens
package hashers

import (
	"fmt"
	"strconv"
	"strings"
)

type HashVersion int

const (
	ScryptVersion HashVersion = iota + 1
	SHA256Version
	SHA3Version
)

// Hasher describes an interface which allows a user to create a hash for a value or verify that a hash is correct.
type Hasher interface {
	// CreateHash creates a hash for a secret, returns nil, err if it encounters an error
	CreateHash(secretKey string) (string, error)
	// VerifyHash verifies that the hash of secretKey == hash
	VerifyHash(hash, secretKey string) error
}

// GetHasherForHash matches a hash with the hasher that produced it by looking at the version in the string.
func GetHasherForHash(hash string) (Hasher, error) {
	version, err := GetHashVersion(hash)
	if err != nil {
		return nil, fmt.Errorf("unable to determine version for hash, %w", err)
	}
	switch HashVersion(version) {
	case ScryptVersion:
		return ScryptHasher{}, nil
	case SHA256Version:
		return Sha256Hasher{}, nil
	case SHA3Version:
		return Sha3Hasher{}, nil
	default:
		return nil, fmt.Errorf("invalid version %d, no hasher exists for that version", version)
	}
}

// GetHasher produces the hasher which should be used for new tokens, for verifying existing tokens use GetHasherForHash.
func GetHasher() Hasher {
	return Sha3Hasher{}
}

// GetHashVersion produces the hash version for a given hash.
func GetHashVersion(hash string) (HashVersion, error) {
	splitHash := strings.SplitN(strings.TrimPrefix(hash, "$"), ":", 3)
	if len(splitHash) != 3 {
		return 0, fmt.Errorf("hash format invalid")
	}
	version, err := strconv.Atoi(splitHash[0])
	// this value could in theory be part of a sensitive value, so we don't include it in the error
	if err != nil {
		return 0, fmt.Errorf("unable to convert hash version")
	}
	return HashVersion(version), nil
}
