package tokens

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

const hashFormat = "$%d:%s:%s" // $version:salt:hash -> $1:abc:def
const Version = 1

// CreateSHA256Hash can be used for basic key hashing, includes a random salt
func CreateSHA256Hash(secretKey string) (string, error) {
	salt := make([]byte, 8)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", salt, secretKey)))
	encSalt := base64.RawStdEncoding.EncodeToString(salt)
	encKey := base64.RawStdEncoding.EncodeToString(hash[:])
	return fmt.Sprintf(hashFormat, Version, encSalt, encKey), nil
}

// VerifySHA256Hash takes a key and compares it with stored hash, including its salt
func VerifySHA256Hash(hash, secretKey string) error {
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
	if version != Version {
		return fmt.Errorf("hash version %d does not match package version %d", version, Version)
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
