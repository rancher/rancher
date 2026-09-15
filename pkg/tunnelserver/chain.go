package tunnelserver

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
)

type Authorizers struct {
	lock      sync.RWMutex
	chain     []remotedialer.Authorizer
	onAuthzed []func(clientKey string)
}

func ErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	fullAddress := req.RemoteAddr
	forwardedFor := req.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		fullAddress = fmt.Sprintf("%s (X-Forwarded-For: %s)", req.RemoteAddr, forwardedFor)
	}
	logrus.Errorf("Failed to handle tunnel request from remote address %s: response %d: %v", fullAddress, code, err)
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
		a.notifyAuthorized(key)
		return key, authed, err
	}

	return "", false, firstErr
}

func (a *Authorizers) Add(authorizer remotedialer.Authorizer) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.chain = append(a.chain, authorizer)
}

// OnAuthorized registers a callback invoked with the client key of every successfully
// authorized tunnel session. remotedialer exposes no session add/remove callback, so this
// is the earliest signal Rancher gets that an agent is connecting.
//
// The callback runs inline on the tunnel request's goroutine and must not block: it fires
// before the session is registered with the tunnel server, so a callback that immediately
// looks the session up will not find it.
func (a *Authorizers) OnAuthorized(f func(clientKey string)) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.onAuthzed = append(a.onAuthzed, f)
}

func (a *Authorizers) notifyAuthorized(clientKey string) {
	a.lock.RLock()
	callbacks := a.onAuthzed
	a.lock.RUnlock()

	for _, f := range callbacks {
		f(clientKey)
	}
}
