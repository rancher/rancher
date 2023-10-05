package systemtokens

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/wrangler/v2/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSystemTokensFromScale(mgmt *config.ScaledContext) systemtokens.Interface {
	return &systemTokens{
		tokenLister: mgmt.Management.Tokens("").Controller().Lister(),
		tokenClient: mgmt.Management.Tokens(""),
	}
}

type systemTokens struct {
	tokenClient v3.TokenInterface
	tokenLister v3.TokenLister
}

// EnsureSystemToken creates tokens or updates their values if they already exist and returns their value.
// TTL defaults to 1 hour, after that this method will auto-refresh. If your token will be in use for more
// than one hour without calling this method again you must pass in an overrideTTL.
// However, if the overrideTTL is 0 the token will never be cleaned up; this should be done with caution.
func (t *systemTokens) EnsureSystemToken(tokenName, description, kind, username string, overrideTTL *int64, randomize bool) (string, error) {
	var err error
	if !randomize {
		_, err = t.tokenLister.Get("", tokenName)
		if err != nil && !apierrors.IsNotFound(err) {
			return "", err
		}
	}

	// needs fresh token because its missing or expired
	val, err := t.createOrUpdateSystemToken(tokenName, description, kind, username, overrideTTL, randomize)
	if err != nil {
		return "", err
	}

	return val, nil
}

func (t *systemTokens) DeleteToken(tokenName string) error {
	return t.tokenClient.Delete(tokenName, &v1.DeleteOptions{})
}

// Creates token obj with hashed token, returns token. Overwrites if pre-existing.
func (t *systemTokens) createOrUpdateSystemToken(tokenName, description, kind, userName string, overrideTTL *int64, randomize bool) (string, error) {
	if strings.HasPrefix(tokenName, "token-") {
		return "", errors.New("token names can't start with token-")
	}
	key, err := randomtoken.Generate()
	if err != nil {
		return "", errors.New("failed to generate token key")
	}

	if !randomize {
		token, err := t.tokenLister.Get("", tokenName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return "", err
			}
		}
		if err == nil {
			if err := t.tokenClient.Delete(token.Name, &v1.DeleteOptions{}); err != nil {
				return "", err
			}
		}
	}

	// not found error, make new token
	token := &v3.Token{
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
	if randomize {
		token.ObjectMeta.Name = ""
		token.ObjectMeta.GenerateName = tokenName
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

	fullVal := fmt.Sprintf("%s:%s", token.Name, key)
	return fullVal, nil
}
