package tunnelserver

import (
	"net/http"

	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
)

type Authorizers struct {
	chain []remotedialer.Authorizer
}

func ErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	logrus.Errorf("Failed to handling tunnel request from %s: response %d: %v", req.RemoteAddr, code, err)
	remotedialer.DefaultErrorWriter(rw, req, code, err)
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
