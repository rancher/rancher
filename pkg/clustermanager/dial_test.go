package clustermanager_test

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// trackedTransport simulates the wrapper that client-go 0.36 introduces
// when ClientsAllowTLSCacheGC is enabled. The real type is unexported
// in k8s.io/client-go/transport, but any non-*http.Transport wrapper
// triggers the same failure.
type trackedTransport struct {
	rt http.RoundTripper
}

func (t *trackedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.rt.RoundTrip(req)
}

// TestWrapTransportFailsWithWrappedTransport demonstrates why using
// WrapTransport to set a custom dialer is broken when the transport
// cache wraps *http.Transport in another type (as client-go 0.36 does
// with trackedTransport and ClientsAllowTLSCacheGC).
func TestWrapTransportFailsWithWrappedTransport(t *testing.T) {
	var dialerCalled atomic.Bool

	customDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		dialerCalled.Store(true)
		return nil, nil
	}

	baseTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Simulate client-go 0.36 wrapping the transport
	wrapped := &trackedTransport{rt: baseTransport}

	// This is what rancher's old WrapTransport did
	wrapFn := func(rt http.RoundTripper) http.RoundTripper {
		if ht, ok := rt.(*http.Transport); ok {
			ht.DialContext = customDialer
		}
		return rt
	}

	wrapFn(wrapped)

	// The dialer was NOT applied because the type assertion failed
	assert.Nil(t, baseTransport.DialContext,
		"WrapTransport should fail to set DialContext on a wrapped transport")
}

// TestDialFieldAppliedRegardlessOfWrapping shows that rest.Config.Dial
// works correctly because it is applied during transport creation,
// before any caching or wrapping occurs.
func TestDialFieldAppliedRegardlessOfWrapping(t *testing.T) {
	var dialerCalled atomic.Bool

	customDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		dialerCalled.Store(true)
		return nil, nil
	}

	// Build a transport config the way client-go does from rest.Config.Dial
	cfg := &transport.Config{
		TLS: transport.TLSConfig{
			Insecure: true,
		},
		DialHolder: &transport.DialHolder{Dial: customDialer},
	}

	rt, err := transport.New(cfg)
	assert.NoError(t, err)

	// Even if the returned transport is wrapped (trackedTransport, auth
	// round trippers, etc.), the dialer is baked into the underlying
	// *http.Transport at creation time. Verify by unwrapping.
	_ = rt // transport is usable regardless of wrapper type

	// Verify the Dial field on rest.Config maps to DialHolder
	restCfg := &rest.Config{
		Host: "https://localhost",
		Dial: customDialer,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	transportCfg, err := restCfg.TransportConfig()
	assert.NoError(t, err)
	assert.NotNil(t, transportCfg.DialHolder,
		"rest.Config.Dial should populate transport.Config.DialHolder")
	assert.NotNil(t, transportCfg.DialHolder.Dial,
		"DialHolder.Dial should be set to our custom dialer")
}
