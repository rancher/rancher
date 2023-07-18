package hashers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicSha256Hash(t *testing.T) {
	secretKey := "hello world"
	hasher := Sha256Hasher{}
	hash, err := hasher.CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)
	// Now check it
	assert.Nil(t, hasher.VerifyHash(hash, secretKey))
	assert.NotNil(t, hasher.VerifyHash(hash, "incorrect"))
}

func TestSha256LongKey(t *testing.T) {
	secretKey := strings.Repeat("A", 720)
	hasher := Sha256Hasher{}
	hash, err := hasher.CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)
	// Now check it
	assert.Nil(t, hasher.VerifyHash(hash, secretKey))
	assert.NotNil(t, hasher.VerifyHash(hash, secretKey+":wrong!"))
}
