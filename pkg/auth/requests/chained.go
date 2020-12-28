package requests

import (
	"net/http"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

func Chain(auths ...Authenticator) Authenticator {
	return &chainedAuth{
		auths: auths,
	}
}

type chainedAuth struct {
	auths []Authenticator
}

func (c *chainedAuth) Authenticate(req *http.Request) (authed bool, user string, groups []string, err error) {
	for _, auth := range c.auths {
		authed, user, groups, err := auth.Authenticate(req)
		if err != nil || authed {
			return authed, user, groups, err
		}
	}
	return false, "", nil, nil
}

func (c *chainedAuth) TokenFromRequest(req *http.Request) (*v3.Token, error) {
	var lastErr error
	for _, auth := range c.auths {
		t, err := auth.TokenFromRequest(req)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
