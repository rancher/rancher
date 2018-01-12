package tokens

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const (
	characters  = "bcdfghjklmnpqrstvwxz2456789"
	tokenLength = 54
)

var charsLength = big.NewInt(int64(len(characters)))

func generateKey() (string, error) {
	token := make([]byte, tokenLength)
	for i := range token {
		r, err := rand.Int(rand.Reader, charsLength)
		if err != nil {
			return "", err
		}
		token[i] = characters[r.Int64()]
	}
	return string(token), nil
}

func getAuthProviderName(principalID string) string {
	parts := strings.Split(principalID, "://")
	return parts[0]
}
