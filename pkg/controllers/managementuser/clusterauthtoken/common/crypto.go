package common

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

const Version = 1
const hashFormat = "$%d:%x:%d:%d:%d:%s"

func CreateHash(secretKey string) (string, error) {
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
	hash := fmt.Sprintf(hashFormat, Version, salt, n, r, p, enc)

	return hash, nil
}

func VerifyHash(hash, secretKey string) error {
	var (
		version, n uint
		r, p       int
		enc        string
		salt       []byte
	)
	_, err := fmt.Sscanf(hash, hashFormat, &version, &salt, &n, &r, &p, &enc)
	if err != nil {
		return err
	}
	if version != Version {
		return fmt.Errorf("hash version %d does not match package version %d", version, Version)
	}

	dk, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return err
	}

	verify, err := scrypt.Key([]byte(secretKey), salt, 1<<n, r, p, len(dk))
	if err != nil {
		return err
	}

	if !bytes.Equal(dk, verify) {
		return fmt.Errorf("secretKey hash does not match")
	}

	return nil
}
