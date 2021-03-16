package tunnelserver

import (
	"net/http"

	"github.com/rancher/remotedialer"
)

type Authorizers struct {
	chain []remotedialer.Authorizer
}

func (a *Authorizers) Authorize(req *http.Request) (clientKey string, authed bool, err error) {
	var (
		firstErr error
	)

	for _, auth := range a.chain {
		key, authed, err := auth(req)
		if err != nil || !authed {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return key, authed, err
	}

	return "", false, firstErr
}

func (a *Authorizers) Add(authorizer remotedialer.Authorizer) {
	a.chain = append(a.chain, authorizer)
}
