package tokens

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicHash(t *testing.T) {
	secretKey := "hello world"
	hash, err := CreateSHA256Hash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)
	// Now check it
	assert.Nil(t, VerifySHA256Hash(hash, secretKey))
	assert.NotNil(t, VerifySHA256Hash(hash, "incorrect"))
}

func TestLongKey(t *testing.T) {
	secretKey := strings.Repeat("A", 720)
	hash, err := CreateSHA256Hash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)
	// Now check it
	assert.Nil(t, VerifySHA256Hash(hash, secretKey))
	assert.NotNil(t, VerifySHA256Hash(hash, secretKey+":wrong!"))
}
