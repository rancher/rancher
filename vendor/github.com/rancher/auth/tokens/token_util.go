package tokens

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
)

func generateKey() (string, error) {
	n := 128
	secretKey := make([]byte, n)
	if _, err := rand.Read(secretKey); err != nil {
		return "", err
	}
	secretKeyString := base64.StdEncoding.EncodeToString(secretKey)
	secretKeyString = sanitizeKey(secretKeyString)

	if len(secretKeyString) < 40 {
		/* Wow, this is terribly bad luck */
		return "", fmt.Errorf("Failed to create secretKey due to not enough good characters")
	}

	return secretKeyString[0:40], nil
}

func sanitizeKey(key string) string {
	re := regexp.MustCompile("[O0lI+/=]")
	key = re.ReplaceAllString(key, "")
	return strings.Trim(key, "")
}

//createTokenWithPayload returns signed jwt token
func createTokenWithPayload(payload map[string]interface{}, secret string) (string, error) {
	token := jwt.New(jwt.GetSigningMethod("HS256"))
	claims := make(jwt.MapClaims)

	for key, value := range payload {
		claims[key] = value
	}

	token.Claims = claims
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		log.Errorf("Failed to sign the token using the secret, error %v", err)
		return "", err
	}
	return signed, nil
}

func EncodeForLabel(input string) string {

	newenc := base64.StdEncoding.WithPadding(base64.NoPadding)
	return newenc.EncodeToString([]byte(input))

}
