package tokens

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if token.TTLMillis != 0 && token.ExpiresAt == "" {
		created := token.ObjectMeta.CreationTimestamp.Time
		ttlDuration, err := time.ParseDuration(strconv.Itoa(token.TTLMillis) + "ms")
		if err != nil {
			logrus.Errorf("Error parsing ttl, cannot calculate expiresAt %v", err)
		} else {
			expiresAtTime := created.Add(ttlDuration)
			token.ExpiresAt = expiresAtTime.UTC().Format(time.RFC3339)
		}
	}
}

func IsNotExpired(token v3.Token) bool {
	if token.TTLMillis == 0 {
		return true
	}
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

func GenerateNewLoginToken(userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerInfo map[string]string, ttl int, description string) v3.Token {
	k8sToken := v3.Token{
		UserPrincipal:   userPrincipal,
		GroupPrincipals: groupPrincipals,
		IsDerived:       false,
		TTLMillis:       ttl,
		UserID:          getUserID(userPrincipal.Name),
		AuthProvider:    getAuthProviderName(userPrincipal.Name),
		ProviderInfo:    providerInfo,
		Description:     description,
	}
	return k8sToken
}

func PurgeExpiredTokens(management *config.ManagementContext) error {
	tokens := management.Management.Tokens("")
	alltokens, err := tokens.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, token := range alltokens.Items {
		expiry, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err != nil {
			continue
		}
		if expiry.Before(time.Now().UTC()) {
			//delete token
			err = tokens.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("Error: %v while deleting expired token: %v", err, token.ObjectMeta.Name)
			}
		}
	}

	return nil
}
