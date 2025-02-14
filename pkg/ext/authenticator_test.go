package ext

import (
	"context"
	"crypto/x509"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

func authenticatorFuncFactory(header string, value string, username string) authenticator.RequestFunc {
	return func(req *http.Request) (*authenticator.Response, bool, error) {
		if slices.Contains(req.Header.Values(header), value) {
			return &authenticator.Response{
				User: &user.DefaultInfo{
					Name: username,
				},
			}, true, nil
		}

		return nil, false, nil
	}
}

var _ dynamiccertificates.ControllerRunner = &controllerRunner{}
var _ dynamiccertificates.CAContentProvider = &controllerRunner{}
var _ authenticator.Request = &controllerRunner{}

type controllerRunner struct {
	isRunning atomic.Bool
}

func (r *controllerRunner) AddListener(listener dynamiccertificates.Listener) {
	panic("unimplemented")
}

func (r *controllerRunner) CurrentCABundleContent() []byte {
	panic("unimplemented")
}

func (r *controllerRunner) Name() string {
	panic("unimplemented")
}

func (r *controllerRunner) VerifyOptions() (x509.VerifyOptions, bool) {
	panic("unimplemented")
}

func (r *controllerRunner) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	return nil, false, nil
}

func (r *controllerRunner) RunOnce(ctx context.Context) error {
	panic("will not be called by ToggleUnionAuthenticator")
}

func (r *controllerRunner) Run(ctx context.Context, nworkers int) {
	r.isRunning.Store(true)
	<-ctx.Done()
	r.isRunning.Store(false)
}

func TestToggleUnionAuthenticator(t *testing.T) {
	f1 := authenticatorFuncFactory("Authorization", "Bearer f1", "f1")
	f2 := authenticatorFuncFactory("Authorization", "Bearer f2", "f2")
	req := &http.Request{
		Header: http.Header{
			"Authorization": []string{"Bearer f1", "Bearer f2"},
		},
	}

	auth := NewToggleUnionAuthenticator()
	auth.Add("f1", f1, false)
	auth.Add("f2", f2, false)

	resp, ok, err := auth.AuthenticateRequest(req)
	assert.Nil(t, resp)
	assert.False(t, ok)
	assert.NoError(t, err)

	auth.SetEnabled("f1", true)
	resp, ok, err = auth.AuthenticateRequest(req)
	assert.Equal(t, "f1", resp.User.GetName())
	assert.True(t, ok)
	assert.NoError(t, err)

	auth.SetEnabled("f1", false)
	auth.SetEnabled("f2", true)
	resp, ok, err = auth.AuthenticateRequest(req)
	assert.Equal(t, "f2", resp.User.GetName())
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestToggleUnionAuthenticatorWithControllerRunners(t *testing.T) {
	a := &controllerRunner{}
	b := &controllerRunner{}

	ctx := context.Background()

	auth := NewToggleUnionAuthenticator()
	auth.Add("a", a, true)
	auth.Add("b", b, true)
	go auth.Run(ctx, 0)

	assert.Eventually(t, func() bool {
		return a.isRunning.Load() && b.isRunning.Load()
	}, time.Second*5, time.Millisecond*100)

	auth.SetEnabled("a", false)
	go auth.Run(ctx, 0)

	assert.Eventually(t, func() bool {
		return !a.isRunning.Load() && b.isRunning.Load()
	}, time.Second*5, time.Millisecond*100)

	auth.SetEnabled("a", true)
	auth.SetEnabled("b", false)
	go auth.Run(ctx, 0)
	assert.Eventually(t, func() bool {
		return a.isRunning.Load() && !b.isRunning.Load()
	}, time.Second*5, time.Millisecond*100)

	auth.SetEnabled("a", false)
	go auth.Run(ctx, 0)
	assert.Eventually(t, func() bool {
		return !a.isRunning.Load() && !b.isRunning.Load()
	}, time.Second*5, time.Millisecond*100)
}
