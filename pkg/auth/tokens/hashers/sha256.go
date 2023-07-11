package hashers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	sha256HashFormat = "$%d:%s:%s" // $version:salt:hash -> $1:abc:def
)

// Sha256Hasher implements the Hasher interface using a backing algorithm of SHA256.
type Sha256Hasher struct{}

// CreateHash hashes secretKey using a random salt and SHA256.
func (s Sha256Hasher) CreateHash(secretKey string) (string, error) {
	salt := make([]byte, 8)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", salt, secretKey)))
	encSalt := base64.RawStdEncoding.EncodeToString(salt)
	encKey := base64.RawStdEncoding.EncodeToString(hash[:])
	return fmt.Sprintf(sha256HashFormat, SHA256Version, encSalt, encKey), nil
}

// VerifyHash compares a key with the hash, and will produce an error if the hash does not match or if the hash is not
// a valid SHA256 hash.
func (s Sha256Hasher) VerifyHash(hash, secretKey string) error {
	if !strings.HasPrefix(hash, "$") {
		return errors.New("hash format invalid")
	}
	splitHash := strings.SplitN(strings.TrimPrefix(hash, "$"), ":", 3)
	if len(splitHash) != 3 {
		return errors.New("hash format invalid")
	}

	version, err := strconv.Atoi(splitHash[0])
	if err != nil {
		return err
	}
	if HashVersion(version) != SHA256Version {
		return fmt.Errorf("hash version %d does not match package version %d", version, SHA256Version)
	}

	salt, enc := splitHash[1], splitHash[2]
	// base64 decode stored salt and key
	decodedKey, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return err
	}
	if len(decodedKey) < 1 {
		return errors.New("secretKey hash does not match") // Don't allow accidental empty string to succeed
	}
	decodedSalt, err := base64.RawStdEncoding.DecodeString(salt)
	if err != nil {
		return err
	}
	// compare the two
	hashedSecretKey := sha256.Sum256([]byte(string(decodedSalt) + secretKey))
	if subtle.ConstantTimeCompare(decodedKey, hashedSecretKey[:]) == 0 {
		return errors.New("secretKey hash does not match")
	}
	return nil
}
