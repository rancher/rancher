package tokens

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
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

func CreateTokenAndSetCookie(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerInfo map[string]string, ttl int, description string, request *types.APIContext) error {
	token, err := NewLoginToken(userID, userPrincipal, groupPrincipals, providerInfo, 0, description)
	if err != nil {
		logrus.Errorf("Failed creating token with error: %v", err)
		return httperror.NewAPIErrorLong(500, "", fmt.Sprintf("Failed creating token with error: %v", err))
	}

	isSecure := false
	if request.Request.URL.Scheme == "https" {
		isSecure = true
	}

	tokenCookie := &http.Cookie{
		Name:     CookieName,
		Value:    token.ObjectMeta.Name + ":" + token.Token,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(request.Response, tokenCookie)
	request.WriteResponse(http.StatusOK, nil)

	return nil
}

func NewLoginToken(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerInfo map[string]string, ttl int64, description string) (v3.Token, error) {
	token := &v3.Token{
		UserPrincipal:   userPrincipal,
		GroupPrincipals: groupPrincipals,
		IsDerived:       false,
		TTLMillis:       ttl,
		UserID:          userID,
		AuthProvider:    getAuthProviderName(userPrincipal.Name),
		ProviderInfo:    providerInfo,
		Description:     description,
	}
	return tokenServer.createK8sTokenCR(token)
}

func UpateLoginToken(token *v3.Token) (*v3.Token, error) {
	return tokenServer.updateK8sTokenCR(token)
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
