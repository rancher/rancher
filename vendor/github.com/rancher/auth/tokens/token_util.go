package tokens

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

func generateKey() (string, error) {
	n := 128
	secretKey := make([]byte, n)
	if _, err := rand.Read(secretKey); err != nil {
		return "", err
	}
	secretKeyString := base64.RawURLEncoding.EncodeToString(secretKey)
	secretKeyString = sanitizeKey(secretKeyString)

	if len(secretKeyString) < 40 {
		/* Wow, this is terribly bad luck */
		return "", fmt.Errorf("Failed to create secretKey due to not enough good characters")
	}

	return secretKeyString[0:40], nil
}

func sanitizeKey(key string) string {
	re := regexp.MustCompile("[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*")
	submatches := re.FindAllString(key, -1)
	submatchJoin := strings.Join(submatches, "")
	submatchJoin = strings.Trim(submatchJoin, "-")
	submatchJoin = strings.Trim(submatchJoin, ".")
	return submatchJoin
}

func getAuthProviderName(principalID string) string {
	parts := strings.Split(principalID, "://")
	return parts[0]
}
