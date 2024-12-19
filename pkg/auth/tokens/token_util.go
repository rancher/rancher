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
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
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

func GetKubeConfigToken(userName, responseType string, userMGR user.Manager, userPrincipal v3.Principal) (*ext.Token, string, error) {
	// create kubeconfig expiring tokens if responseType=kubeconfig in login action vs login tokens for responseType=json
	clusterID := extractClusterIDFromResponseType(responseType)

	logrus.Debugf("getKubeConfigToken: responseType %s", responseType)
	name := "kubeconfig-" + userName
	if clusterID != "" {
		name = fmt.Sprintf("kubeconfig-%s.%s", userName, clusterID)
	}

	token, tokenVal, err := userMGR.GetKubeconfigToken(clusterID, name, "Kubeconfig token", "kubeconfig", userName, userPrincipal)
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
func VerifyToken(storedToken *v3.Token, tokenName, tokenKey string) (int, error) {
	invalidAuthTokenErr := errors.New("Invalid auth token value")
	if storedToken == nil || storedToken.Name != tokenName {
		return http.StatusUnprocessableEntity, invalidAuthTokenErr
	}
	if storedToken.Annotations != nil && storedToken.Annotations[TokenHashed] == "true" {
		hasher, err := hashers.GetHasherForHash(storedToken.Token)
		if err != nil {
			logrus.Errorf("unable to get a hasher for token with error %v", err)
			return http.StatusInternalServerError, fmt.Errorf("unable to verify hash")
		}
		if err := hasher.VerifyHash(storedToken.Token, tokenKey); err != nil {
			logrus.Errorf("VerifyHash failed with error: %v", err)
			return http.StatusUnprocessableEntity, invalidAuthTokenErr
		}
	} else {
		if storedToken.Token != tokenKey {
			return http.StatusUnprocessableEntity, invalidAuthTokenErr
		}
	}
	if IsExpired(*storedToken) {
		return http.StatusGone, errors.New("must authenticate")
	}
	return http.StatusOK, nil
}

// ConvertTokenKeyToHash takes a token with an un-hashed key and converts it to a hashed key
func ConvertTokenKeyToHash(token *v3.Token) error {
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

// Given a stored token with hashed key, check if the provided (unhashed) tokenKey matches and is valid
func ExtVerifyToken(storedToken *ext.Token, tokenName, tokenKey string) (int, error) {
	invalidAuthTokenErr := errors.New("Invalid auth token value")

	if storedToken == nil || storedToken.ObjectMeta.Name != tokenName {
		return http.StatusUnprocessableEntity, invalidAuthTokenErr
	}

	// Ext token always has a hash. Only a hash.

	hasher, err := hashers.GetHasherForHash(storedToken.Status.TokenHash)
	if err != nil {
		logrus.Errorf("unable to get a hasher for token with error %v", err)
		return http.StatusInternalServerError,
			fmt.Errorf("unable to verify hash '%s'", storedToken.Status.TokenHash)
	}

	if err := hasher.VerifyHash(storedToken.Status.TokenHash, tokenKey); err != nil {
		logrus.Errorf("VerifyHash failed with error: %v", err)
		return http.StatusUnprocessableEntity, invalidAuthTokenErr
	}

	if storedToken.Status.Expired {
		return http.StatusGone, errors.New("must authenticate")
	}
	return http.StatusOK, nil
}

// ExtConvertTokenResource converts an ext token into a semblance of what ConvertTokenResource
// returns for a norman token.
func ExtConvertTokenResource(token ext.Token) (map[string]interface{}, error) {
	// The set of fields to create were empirically pulled out of the rancher logs, using a
	// modified rancher dumping the ConvertTokenResource result.

	// Example
	// token data [.selfLink] = string ((/apis/management.cattle.io/v3/tokens/token-rv2xw))
	// token data [authProvider] = string ((local))
	// token data [created] = string ((2024-12-10T13:22:04Z))
	// token data [current] = bool ((false))
	// token data [description] = string ((foo))
	// token data [expired] = bool ((false))
	// token data [expiresAt] = string (())
	// token data [id] = string ((token-rv2xw))
	// token data [isDerived] = bool ((true))
	// token data [labels] = map[string]interface {} ((map[authn.management.cattle.io/token-userId:user-cl9vb cattle.io/creator:norman]))
	// token data [name] = string ((token-rv2xw))
	// token data [state] = string ((active))
	// token data [token] = string ((fwrl2p749slsbghmxlpjwhf4wvxlrz8r26wzclhwnknhp4sf9vdjsm))
	// token data [transitioningMessage] = string (())
	// token data [transitioning] = string ((no))
	// token data [ttl] = json.Number ((7776000000))
	// token data [type] = string ((/v3/schemas/token))
	// token data [userId] = string ((user-cl9vb))
	// token data [userPrincipal] = map[string]interface {} ((map[displayName:Default Admin loginName:admin me:true metadata:map[creationTimestamp:<nil> name:local://user-cl9vb] principalType:user provider:local]
	// token data [uuid] = string ((a5758481-0d54-48f2-b5bd-1347c2f0d946))

	tokenData := map[string]interface{}{}

	// TODO tokenData[".selfLink"] =
	// TODO tokenData["type"] = ??
	tokenData["authProvider"] = token.GetAuthProvider()
	tokenData["created"] = token.ObjectMeta.CreationTimestamp
	tokenData["current"] = false
	tokenData["description"] = token.Spec.Description
	tokenData["expired"] = token.Status.Expired
	tokenData["expiredAt"] = token.Status.ExpiresAt
	tokenData["id"] = token.GetName()
	tokenData["isDerived"] = token.GetIsDerived()
	tokenData["labels"] = token.ObjectMeta.Labels // FIX ? getting a map[string]interface{}
	tokenData["name"] = token.GetName()
	tokenData["state"] = "active" // TODO FIX - use enabled. string for "not active" state is not known
	tokenData["token"] = ""       // generally not available
	tokenData["transitioningMessage"] = ""
	tokenData["transitioning"] = "no"
	tokenData["ttl"] = token.Spec.TTL
	tokenData["userId"] = token.GetUserID()
	tokenData["userPrincipal"] = token.GetUserPrincipal() // FIX ? getting a map[string]interface{}
	tokenData["uuid"] = token.ObjectMeta.UID

	return tokenData, nil
}
