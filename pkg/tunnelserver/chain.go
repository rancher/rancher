package tunnelserver

import (
	"fmt"
	"net/http"

	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
)

type Authorizers struct {
	chain []remotedialer.Authorizer
}

func ErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	fullAddress := req.RemoteAddr
	forwardedFor := req.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		fullAddress = fmt.Sprintf("%s (X-Forwarded-For: %s)", req.RemoteAddr, forwardedFor)
	}
	logrus.Errorf("Failed to handle tunnel request from remote address %s: response %d: %v", fullAddress, code, err)
	logrus.Tracef("ErrorWriter: response code: %d, request: %v", code, req)
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
