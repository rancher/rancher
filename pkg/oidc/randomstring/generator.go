package randomstring

import (
	"crypto/rand"
	"math/big"
)

const (
	characters         = "bcdfghjklmnpqrstvwxz2456789"
	clientIDLength     = 10
	codeLength         = 56
	clientSecretLength = 56
	clientIDPrefix     = "client-"
	codePrefix         = "code-"
	clientSecretPrefix = "secret-"
)

type Generator struct{}

var charsLength = big.NewInt(int64(len(characters)))

// GenerateClientID generates an OIDC Client ID. It has 'client-' as a prefix and 10 random characters.
func (r *Generator) GenerateClientID() (string, error) {
	return r.generateRandomString(clientIDPrefix, clientIDLength)
}

// GenerateClientSecret generates an OIDC Client Secret. It has 'secret-' as a prefix and 56 random characters.
func (r *Generator) GenerateClientSecret() (string, error) {
	return r.generateRandomString(clientSecretPrefix, clientSecretLength)
}

// GenerateCode generates an OIDC Code. It has 'code-' as a prefix and 56 random characters.
func (r *Generator) GenerateCode() (string, error) {
	return r.generateRandomString(codePrefix, codeLength)
}

func (r *Generator) generateRandomString(prefix string, length int) (string, error) {
	token := make([]byte, length)
	for i := range token {
		r, err := rand.Int(rand.Reader, charsLength)
		if err != nil {
			return "", err
		}
		token[i] = characters[r.Int64()]
	}
	return prefix + string(token), nil
}
