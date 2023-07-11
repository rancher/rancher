package hashers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicScryptHash(t *testing.T) {
	secretKey := "hello world"
	hasher := ScryptHasher{}
	hash, err := hasher.CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)
	// Now check it
	assert.Nil(t, hasher.VerifyHash(hash, secretKey))
	assert.NotNil(t, hasher.VerifyHash(hash, "incorrect"))
}

func TestScryptLongKey(t *testing.T) {
	secretKey := strings.Repeat("A", 720)
	hasher := ScryptHasher{}
	hash, err := hasher.CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)
	// Now check it
	assert.Nil(t, hasher.VerifyHash(hash, secretKey))
	assert.NotNil(t, hasher.VerifyHash(hash, secretKey+":wrong!"))
}
