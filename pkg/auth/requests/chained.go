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

func (c *chainedAuth) Authenticate(req *http.Request) (*AuthenticatorResponse, error) {
	for _, auth := range c.auths {
		authResponse, err := auth.Authenticate(req)
		if err != nil || authResponse.IsAuthed {
			return authResponse, err
		}
	}
	return &AuthenticatorResponse{
		Extras: make(map[string][]string),
	}, nil
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
