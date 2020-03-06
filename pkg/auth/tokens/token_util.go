package tokens

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

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

func SetTokenExpiresAt(token *v3.Token) {
	if token.TTLMillis != 0 {
		created := token.ObjectMeta.CreationTimestamp.Time
		ttlDuration := time.Duration(token.TTLMillis) * time.Millisecond
		expiresAtTime := created.Add(ttlDuration)
		token.ExpiresAt = expiresAtTime.UTC().Format(time.RFC3339)
	}
}

func IsExpired(token v3.Token) bool {
	if token.TTLMillis == 0 {
		return false
	}

	created := token.ObjectMeta.CreationTimestamp.Time
	durationElapsed := time.Since(created)

	ttlDuration := time.Duration(token.TTLMillis) * time.Millisecond
	return durationElapsed.Seconds() >= ttlDuration.Seconds()
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

func ConvertTokenResource(schema *types.Schema, token v3.Token) (map[string]interface{}, error) {
	tokenData, err := convert.EncodeToMap(token)
	if err != nil {
		return nil, err
	}
	mapper := schema.Mapper
	if mapper == nil {
		return nil, errors.New("no schema mapper available")
	}
	mapper.FromInternal(tokenData)

	return tokenData, nil
}

// Given a stored token with hashed key, check if the provided (unhashed) tokenKey matches and is valid
func VerifyToken(storedToken *v3.Token, tokenName, tokenKey string) (int, error) {
	if storedToken.ObjectMeta.Name != tokenName {
		return 422, errors.New("Invalid auth token value")
	}
	if err := VerifySHA256Hash(storedToken.Token, tokenKey); err != nil {
		logrus.Errorf("VerifySHA256Hash failed with error: %v", err)
		return 422, errors.New("Invalid auth token value")
	}
	if IsExpired(*storedToken) {
		return 410, errors.New("must authenticate")
	}
	return 200, nil
}

// ConvertTokenKeyToHash takes a token with an un-hashed key and converts it to a hashed key
func ConvertTokenKeyToHash(token *v3.Token) error {
	if token != nil && len(token.Token) > 0 {
		hashedToken, err := CreateSHA256Hash(token.Token)
		if err != nil {
			logrus.Errorf("Failed to generate hash from token: %v", err)
			return errors.New("failed to generate hash from token")
		}
		token.Token = hashedToken
		if token.Annotations == nil {
			token.Annotations = map[string]string{}
		}
		token.Annotations[TokenHashed] = "true"
	}
	return nil
}
