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

const (
	authHeaderName  = "Authorization"
	authValuePrefix = "Bearer "
)

func (a *tokenAuthenticator) Authenticate(req *http.Request) (bool, string, []string, error) {
	//check if token cookie or authorization header

	var tokenID string

	authHeader := req.Header.Get(authHeaderName)
	authHeader = strings.TrimPrefix(authHeader, " ")

	if authHeader != "" && strings.HasPrefix(authHeader, authValuePrefix) {
		logrus.Debugf("Authenticate: auth header: %v", authHeader)
		tokenID = strings.TrimPrefix(authHeader, authValuePrefix)
		tokenID = strings.TrimSpace(tokenID)
	} else {
		cookie, err := req.Cookie(tokens.CookieName)
		if err == nil {
			logrus.Debugf("Authenticate: token cookie: %v %v", cookie.Name, cookie.Value)
			tokenID = cookie.Value
		}
	}

	logrus.Debugf("Authenticate: token ID: %v", tokenID)

	if tokenID == "" {
		// no cookie or auth header, cannot authenticate
		return false, "", []string{}, fmt.Errorf("failed to find auth cookie or headers")
	}

	token, err := a.getTokenCR(tokenID)
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
		return nil, fmt.Errorf("failed to retrieve auth token, error: %v", err)
	}

	logrus.Debugf("storedToken token resource: %v", storedToken)

	return storedToken, nil
}
