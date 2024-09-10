package hashers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptHashFormat = "$%d:%x:%d:%d:%d:%s"
)

// ScryptHasher implements the Hasher interface using a backing alogorithm of Scrypt.
type ScryptHasher struct{}

// CreateHash hahshes secretKey using a salt and scrypt.
func (s ScryptHasher) CreateHash(secretKey string) (string, error) {
	const (
		n       = 15
		r       = 8
		p       = 1
		keyLen  = 64
		saltLen = 8
	)
	salt := make([]byte, saltLen)

	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	dk, err := scrypt.Key([]byte(secretKey), salt, 1<<n, r, p, keyLen)
	if err != nil {
		return "", err
	}

	enc := base64.RawStdEncoding.EncodeToString(dk)
	hash := fmt.Sprintf(scryptHashFormat, ScryptVersion, salt, n, r, p, enc)

	return hash, nil
}

// VerifyHash compares a key with the hash, and will produce an error if the hash does not match or if the hash is not
// a valid scrypt hash.
func (s ScryptHasher) VerifyHash(hash, secretKey string) error {
	var (
		version, n uint
		r, p       int
		enc        string
		salt       []byte
	)
	_, err := fmt.Sscanf(hash, scryptHashFormat, &version, &salt, &n, &r, &p, &enc)
	if err != nil {
		return err
	}
	if HashVersion(version) != ScryptVersion {
		return fmt.Errorf("hash version %d does not match package version %d", version, ScryptVersion)
	}

	dk, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return err
	}

	verify, err := scrypt.Key([]byte(secretKey), salt, 1<<n, r, p, len(dk))
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(dk, verify) == 0 {
		return fmt.Errorf("secretKey hash does not match")
	}

	return nil
}
