package tokens

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/sirupsen/logrus"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
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
	externalType := parts[0]
	providerParts := strings.Split(externalType, "_")
	return providerParts[0]
}

func getUserID(principalID string) string {
	parts := strings.Split(principalID, "://")
	return parts[1]
}

func SplitTokenParts(tokenID string) (string, string) {
	parts := strings.Split(tokenID, ":")
	if len(parts) != 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func IsNotExpired(token v3.Token) bool {
	created := token.ObjectMeta.CreationTimestamp.Time
	durationElapsed := time.Since(created)

	ttlDuration, err := time.ParseDuration(strconv.Itoa(token.TTLMillis) + "ms")
	if err != nil {
		logrus.Errorf("Error parsing ttl %v", err)
		return false
	}

	if durationElapsed.Seconds() <= ttlDuration.Seconds() {
		return true
	}
	return false
}

func GetTokenAuthFromRequest(req *http.Request) string {
	var tokenAuthValue string
	authHeader := req.Header.Get(AuthHeaderName)
	authHeader = strings.TrimSpace(authHeader)

	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if strings.EqualFold(parts[0], AuthValuePrefix) {
			if len(parts) > 1 {
				tokenAuthValue = strings.TrimSpace(parts[1])
			}
		} else if strings.EqualFold(parts[0], BasicAuthPrefix) {
			if len(parts) > 1 {
				base64Value := strings.TrimSpace(parts[1])
				data, err := base64.URLEncoding.DecodeString(base64Value)
				if err != nil {
					logrus.Errorf("Error %v parsing %v header", err, AuthHeaderName)
				} else {
					tokenAuthValue = string(data)
				}
			}
		}
	} else {
		cookie, err := req.Cookie(CookieName)
		if err == nil {
			tokenAuthValue = cookie.Value
		}
	}
	return tokenAuthValue
}
