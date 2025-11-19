package tokens

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

var (
	errInvalidAuthToken = errors.New("invalid auth token value")
)

func SplitTokenParts(tokenID string) (string, string) {
	parts := strings.Split(tokenID, ":")
	if len(parts) != 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func SetTokenExpiresAt(token *apiv3.Token) {
	if token.TTLMillis != 0 {
		created := token.ObjectMeta.CreationTimestamp.Time
		ttlDuration := time.Duration(token.TTLMillis) * time.Millisecond
		expiresAtTime := created.Add(ttlDuration)
		token.ExpiresAt = expiresAtTime.UTC().Format(time.RFC3339)
	}
}

// IsExpired returns true if the token is expired.
func IsExpired(token accessor.TokenAccessor) bool {
	return token.GetIsExpired()
}

// IsIdleExpired returns true if last recorded user activity is past the idle timeout.
func IsIdleExpired(token accessor.TokenAccessor, now time.Time) bool {
	activityLastSeen := token.GetLastActivitySeen()

	if activityLastSeen.IsZero() {
		return false
	}

	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()
	if idleTimeout == 0 {
		return false
	}

	return now.After(activityLastSeen.Add(time.Duration(idleTimeout) * time.Minute))
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

func ConvertTokenResource(schema *types.Schema, token apiv3.Token) (map[string]interface{}, error) {
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

type kubeconfigTokenGetter interface {
	GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal apiv3.Principal) (*apiv3.Token, string, error)
}

func GetKubeConfigToken(userName, responseType string, kubeconfigTokenGetter kubeconfigTokenGetter, userPrincipal apiv3.Principal) (*apiv3.Token, string, error) {
	// create kubeconfig expiring tokens if responseType=kubeconfig in login action vs login tokens for responseType=json
	clusterID := extractClusterIDFromResponseType(responseType)

	logrus.Debugf("getKubeConfigToken: responseType %s", responseType)
	name := "kubeconfig-" + userName
	if clusterID != "" {
		name = fmt.Sprintf("kubeconfig-%s.%s", userName, clusterID)
	}

	token, tokenVal, err := kubeconfigTokenGetter.GetKubeconfigToken(clusterID, name, "Kubeconfig token", "kubeconfig", userName, userPrincipal)
	if err != nil {
		return nil, "", err
	}

	return token, tokenVal, nil
}

func extractClusterIDFromResponseType(responseType string) string {
	responseSplit := strings.SplitN(responseType, "_", 2)
	if len(responseSplit) != 2 {
		return ""
	}
	return responseSplit[1]
}

// Given a stored token with hashed key, check if the provided (unhashed) tokenKey matches and is valid
func VerifyToken(storedToken *apiv3.Token, tokenName, tokenKey string) (int, error) {
	if storedToken == nil || storedToken.Name != tokenName {
		return http.StatusUnprocessableEntity, errInvalidAuthToken
	}

	if storedToken.Annotations != nil && storedToken.Annotations[TokenHashed] == "true" {
		hasher, err := hashers.GetHasherForHash(storedToken.Token)
		if err != nil {
			logrus.Errorf("unable to get a hasher for token with error %v", err)
			return http.StatusInternalServerError, fmt.Errorf("unable to verify hash")
		}
		if err := hasher.VerifyHash(storedToken.Token, tokenKey); err != nil {
			logrus.Errorf("VerifyHash failed with error: %v", err)
			return http.StatusUnprocessableEntity, errInvalidAuthToken
		}
	} else {
		if storedToken.Token != tokenKey {
			return http.StatusUnprocessableEntity, errInvalidAuthToken
		}
	}

	if IsExpired(storedToken) {
		return http.StatusGone, errors.New("must authenticate, expired")
	}

	if IsIdleExpired(storedToken, time.Now()) {
		return http.StatusGone, errors.New("must authenticate, session idle timeout expired")
	}

	return http.StatusOK, nil
}

// ConvertTokenKeyToHash takes a token with an un-hashed key and converts it to a hashed key
func ConvertTokenKeyToHash(token *apiv3.Token) error {
	if !features.TokenHashing.Enabled() {
		return nil
	}
	if token != nil && len(token.Token) > 0 {
		hasher := hashers.GetHasher()
		hashedToken, err := hasher.CreateHash(token.Token)
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
