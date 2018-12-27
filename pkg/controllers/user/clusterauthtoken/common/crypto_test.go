package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicHash(t *testing.T) {
	secretKey := "hello world"
	hash, err := CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)

	assert.Nil(t, VerifyHash(hash, secretKey))
	assert.NotNil(t, VerifyHash(hash, "goodbye"))
}

func TestLongKey(t *testing.T) {
	secretKey := strings.Repeat("A", 720)
	hash, err := CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)

	assert.Nil(t, VerifyHash(hash, secretKey))
	assert.NotNil(t, VerifyHash(hash, secretKey+":wrong!"))
}
