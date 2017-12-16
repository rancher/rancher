package authenticator

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type tokenAuthenticator struct {
	ctx    context.Context
	tokens v3.TokenLister
}

func (a *tokenAuthenticator) Authenticate(req *http.Request) (bool, string, []string, error) {
	cookie, err := req.Cookie(tokens.CookieName)
	if err != nil {
		return false, "", []string{}, fmt.Errorf("Failed to find auth cookie")
	}
	logrus.Debugf("Authenticate: token cookie: %v %v", cookie.Name, cookie.Value)

	token, err := a.getTokenCR(cookie.Value)
	if err != nil {
		return false, "", []string{}, err
	}

	var groups []string
	for _, principal := range token.GroupPrincipals {
		// TODO This is a short cut for now. Will actually need to lookup groups in future
		name := strings.TrimPrefix(principal.Name, "local://")
		groups = append(groups, name)
	}

	return true, token.UserID, groups, nil
}

func (a *tokenAuthenticator) getTokenCR(tokenID string) (*v3.Token, error) {
	storedToken, err := a.tokens.Get("", tokenID)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve auth token, error: %v", err)
	}

	logrus.Debugf("storedToken token resource: %v", storedToken)

	return storedToken, nil
}
