package systemtokens

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/randomtoken"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSystemTokensFromScale(mgmt *config.ScaledContext) *SystemTokens {
	return &SystemTokens{
		tokenLister:  mgmt.Management.Tokens("").Controller().Lister(),
		tokenClient:  mgmt.Management.Tokens(""),
		tokenCache:   map[string]string{},
		isHA:         mgmt.PeerManager != nil,
		haIdentifier: getIdentifier(),
	}
}

type SystemTokens struct {
	isHA            bool
	tokenClient     v3.TokenInterface
	tokenLister     v3.TokenLister
	haIdentifier    string
	tokenCache      map[string]string
	tokenCacheMutex sync.RWMutex
}

// Multiple rancher management servers might attempt to use the same key name
// Because the token is hashed, any regeneration of an existing token name invalidates all previous tokens
// So to avoid those HA bugs, we need to uniquely create and re-use each token for each
// requesting management server, which we do here by generating a random string.
func getIdentifier() string {
	randString, err := randomtoken.Generate()
	if err != nil {
		return strconv.Itoa(rand.Intn(10000))
	}
	return randString[0:5]
}

// EnsureSystemToken gets or creates tokens for management use, and keeps them in memory inside contexts.
// Appends identifier to key name
// TTL defaults to 1 hour, after that this method will auto-refresh. If your token will be in use for more
// than one hour without calling this method again you must pass in an overrideTTL.
// However, the overrideTTL must not be 0, otherwise the token will never be cleaned up.
func (t *SystemTokens) EnsureSystemToken(key, description, kind, username string, overrideTTL *int64) (string, error) {
	if overrideTTL != nil && *overrideTTL == 0 {
		return "", errors.New("TTL for system token must not be zero") // no way to cleanup token
	}
	key = fmt.Sprintf("%s-%s", key, t.haIdentifier) // append hashed identifier, see getIdentifier
	token, err := t.tokenLister.Get("", key)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}

	if err == nil && !tokens.IsExpired(*token) {
		t.tokenCacheMutex.RLock()
		val, ok := t.tokenCache[key]
		t.tokenCacheMutex.RUnlock()
		if ok {
			return val, nil
		}
	}
	// needs fresh token because its missing or expired
	val, err := t.createOrUpdateSystemToken(key, description, kind, username, overrideTTL)
	if err != nil {
		return "", err
	}

	t.tokenCacheMutex.Lock()
	defer t.tokenCacheMutex.Unlock()
	t.tokenCache[key] = val
	return val, nil
}

// Creates token obj with hashed token, returns token. Overwrites if pre-existing.
func (t *SystemTokens) createOrUpdateSystemToken(tokenName, description, kind, userName string, overrideTTL *int64) (string, error) {
	if strings.HasPrefix(tokenName, "token-") {
		return "", errors.New("token names can't start with token-")
	}
	key, err := randomtoken.Generate()
	if err != nil {
		return "", errors.New("failed to generate token key")
	}

	token, err := t.tokenLister.Get("", tokenName)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}
	if token != nil {
		token.Token = key
		err = tokens.ConvertTokenKeyToHash(token)
		if err != nil {
			return "", err
		}
		logrus.Infof("Updating system token for %v, token: %v", userName, tokenName)
		token, err = t.tokenClient.Update(token)
		if err != nil {
			return "", err
		}
	} else {
		token = &v3.Token{
			ObjectMeta: v1.ObjectMeta{
				Name: tokenName,
				Labels: map[string]string{
					tokens.UserIDLabel:    userName,
					tokens.TokenKindLabel: kind,
				},
			},
			TTLMillis:    3600000, // 1 hour, token purge daemon will cleanup
			Description:  description,
			UserID:       userName,
			AuthProvider: "local",
			IsDerived:    true,
			Token:        key,
		}
		if overrideTTL != nil {
			token.TTLMillis = *overrideTTL
		}
		if t.isHA {
			if token.Annotations == nil {
				token.Annotations = make(map[string]string)
			}
			// For debugging purposes we set hostname as annotation, which in HA is the pod name
			token.Annotations[tokens.HAIdentifier] = os.Getenv("HOSTNAME")
		}

		err = tokens.ConvertTokenKeyToHash(token)
		if err != nil {
			return "", err
		}
		logrus.Infof("Creating system token for %v, token: %v", userName, tokenName)
		token, err = t.tokenClient.Create(token)
		if err != nil {
			return "", err
		}
	}
	return token.Name + ":" + key, nil
}
