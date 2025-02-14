package ext

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	authenticatorunion "k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

const (
	authenticatorNameSteveDefault = "default"
	authenticatorNameRancherUser  = "rancher-user"
)

type toggle[T any] struct {
	enabled bool
	value   T
}

var _ authenticator.Request = &ToggleUnionAuthenticator{}
var _ dynamiccertificates.CAContentProvider = &ToggleUnionAuthenticator{}

type ToggleUnionAuthenticator struct {
	authenticatorsMu       sync.RWMutex
	authenticators         map[string]toggle[authenticator.Request]
	unionAuthenticator     authenticator.Request
	unionCAContentProvider dynamiccertificates.CAContentProvider

	runnerMu         sync.RWMutex
	runnerCtxCancels []context.CancelFunc
}

func NewToggleUnionAuthenticator() *ToggleUnionAuthenticator {
	a := &ToggleUnionAuthenticator{
		authenticators: make(map[string]toggle[authenticator.Request]),
	}

	a.sync()

	return a
}

// sync synchronizes the internal collection of authenticators and stops and cancels any existing calls to Run.
func (a *ToggleUnionAuthenticator) sync() {
	a.authenticatorsMu.Lock()
	a.runnerMu.Lock()
	defer a.runnerMu.Unlock()
	defer a.authenticatorsMu.Unlock()

	logrus.Infof("syncing imperative api authentication providers")

	var authenticators []authenticator.Request
	var caProviders []dynamiccertificates.CAContentProvider

	for _, toggle := range a.authenticators {
		if !toggle.enabled {
			continue
		}

		authenticators = append(authenticators, toggle.value)

		if provider, ok := toggle.value.(dynamiccertificates.CAContentProvider); ok {
			caProviders = append(caProviders, provider)
		}
	}

	a.unionAuthenticator = authenticatorunion.New(authenticators...)
	a.unionCAContentProvider = dynamiccertificates.NewUnionCAContentProvider(caProviders...)

	for _, cancel := range a.runnerCtxCancels {
		cancel()
	}

	a.runnerCtxCancels = []context.CancelFunc{}
}

func (a *ToggleUnionAuthenticator) SetEnabled(name string, enabled bool) error {
	defer a.sync()

	a.authenticatorsMu.Lock()
	a.authenticatorsMu.Unlock()

	toggle, ok := a.authenticators[name]
	if !ok {
		return fmt.Errorf("no authenticator found with name '%s'", name)
	}

	toggle.enabled = enabled
	a.authenticators[name] = toggle

	return nil
}

func (a *ToggleUnionAuthenticator) Add(name string, auth authenticator.Request, enabled bool) {
	defer a.sync()

	a.authenticatorsMu.Lock()
	defer a.authenticatorsMu.Unlock()

	a.authenticators[name] = toggle[authenticator.Request]{
		enabled: enabled,
		value:   auth,
	}
}

func (a *ToggleUnionAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	a.authenticatorsMu.RLock()
	defer a.authenticatorsMu.RUnlock()

	return a.unionAuthenticator.AuthenticateRequest(req)
}

// AddListener implements dynamiccertificates.CAContentProvider.
func (a *ToggleUnionAuthenticator) AddListener(listener dynamiccertificates.Listener) {
	a.authenticatorsMu.RLock()
	defer a.authenticatorsMu.RUnlock()

	a.unionCAContentProvider.AddListener(listener)
}

// CurrentCABundleContent implements dynamiccertificates.CAContentProvider.
func (a *ToggleUnionAuthenticator) CurrentCABundleContent() []byte {
	a.authenticatorsMu.RLock()
	defer a.authenticatorsMu.RUnlock()

	return a.unionCAContentProvider.CurrentCABundleContent()
}

// Name implements dynamiccertificates.CAContentProvider.
func (a *ToggleUnionAuthenticator) Name() string {
	a.authenticatorsMu.RLock()
	defer a.authenticatorsMu.RUnlock()

	return a.unionCAContentProvider.Name()
}

// VerifyOptions implements dynamiccertificates.CAContentProvider.
func (a *ToggleUnionAuthenticator) VerifyOptions() (x509.VerifyOptions, bool) {
	a.authenticatorsMu.RLock()
	defer a.authenticatorsMu.RUnlock()

	return a.unionCAContentProvider.VerifyOptions()
}

func (a *ToggleUnionAuthenticator) Run(ctx context.Context, workers int) {
	a.runnerMu.Lock()
	defer a.runnerMu.Unlock()

	runner, ok := a.unionCAContentProvider.(dynamiccertificates.ControllerRunner)
	if !ok {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	a.runnerCtxCancels = append(a.runnerCtxCancels, cancel)

	runner.Run(ctx, workers)
}
