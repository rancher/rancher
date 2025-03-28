package randomstring

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateClientID(t *testing.T) {
	g := Generator{}

	clientID, err := g.GenerateClientID()

	assert.NoError(t, err)
	assert.True(t, len(clientID) == 17)
	assert.True(t, strings.HasPrefix(clientID, clientIDPrefix))
}

func TestGenerateClientSecret(t *testing.T) {
	g := Generator{}

	clientSecret, err := g.GenerateClientSecret()

	assert.NoError(t, err)
	assert.True(t, len(clientSecret) == 63)
	assert.True(t, strings.HasPrefix(clientSecret, clientSecretPrefix))
}

func TestGenerateCode(t *testing.T) {
	g := Generator{}

	code, err := g.GenerateCode()

	assert.NoError(t, err)
	assert.True(t, len(code) == 61)
	assert.True(t, strings.HasPrefix(code, codePrefix))
}
