package httpproxy

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// recordingTransport tracks whether it was called and returns a preset error.
type recordingTransport struct {
	called bool
	err    error
}

func (r *recordingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.called = true
	return nil, r.err
}

// endpointWithRoute builds a minimal ProxyEndpoint with a single route.
func endpointWithRoute(domain string, insecure bool, injection *mgmt.CredentialInjectionSpec) *mgmt.ProxyEndpoint {
	return &mgmt.ProxyEndpoint{
		Spec: mgmt.ProxyEndpointSpec{
			Routes: []mgmt.ProxyEndpointRoute{
				{
					Domain:                domain,
					InsecureSkipTLSVerify: insecure,
					CredentialInjection:   injection,
				},
			},
		},
	}
}

// --- routeMatchesHost ---

func TestRouteMatchesHost(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		host    string
		want    bool
	}{
		{"exact match", "api.example.com", "api.example.com", true},
		{"exact mismatch", "api.example.com", "api.other.com", false},
		{"star wildcard match", "*example.com", "prefixexample.com", true},
		{"star wildcard subdomain", "*.example.com", "sub.example.com", true},
		{"star wildcard no match", "*.example.com", "api.other.com", false},
		{"percent wildcard match", "ec2.%.amazonaws.com", "ec2.us-west-2.amazonaws.com", true},
		{"percent wildcard no match", "ec2.%.amazonaws.com", "ec2.us-west-2.other.com", false},
		{"overly broad wildcard skipped", "*.com", "anything.com", false},
		{"empty pattern", "", "example.com", false},
		{"empty host", "example.com", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, routeMatchesHost(tt.pattern, tt.host))
		})
	}
}

// --- findMatchingRoute ---

func TestFindMatchingRoute_NoEndpoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return(nil, nil)

	p := &proxy{proxyEndpointCache: cache}
	assert.Nil(t, p.findMatchingRoute("api.example.com"))
}

func TestFindMatchingRoute_NoMatchingRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute("api.other.com", false, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache}
	assert.Nil(t, p.findMatchingRoute("api.example.com"))
}

func TestFindMatchingRoute_ReturnsMatchingRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute("api.other.com", false, nil),
		endpointWithRoute("api.example.com", true, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache}
	route := p.findMatchingRoute("api.example.com")
	require.NotNil(t, route)
	assert.Equal(t, "api.example.com", route.Domain)
	assert.True(t, route.InsecureSkipTLSVerify)
}

func TestFindMatchingRoute_WildcardDomain(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute("*.example.com", false, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache}
	route := p.findMatchingRoute("sub.example.com")
	require.NotNil(t, route)
	assert.Equal(t, "*.example.com", route.Domain)
}

func TestFindMatchingRoute_SkipsOverlyBroadDomain(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute("*.com", false, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache}
	assert.Nil(t, p.findMatchingRoute("anything.com"), "overly broad domain must not match")
}

func TestFindMatchingRoute_CacheError_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return(nil, errors.New("cache unavailable"))

	p := &proxy{proxyEndpointCache: cache}
	assert.Nil(t, p.findMatchingRoute("api.example.com"), "cache error should return nil, not panic")
}

// --- applyRouteInjection ---

func TestApplyRouteInjection_MissingCredID(t *testing.T) {
	p := &proxy{}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/path", nil)
	route := &mgmt.ProxyEndpointRoute{
		CredentialInjection: &mgmt.CredentialInjectionSpec{Mode: "bearer", TokenField: "token"},
	}

	// cAuth has no credID param; getRequestParams parses nothing from a single token.
	err := p.applyRouteInjection(req, "inject", route)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credID")
}

func TestApplyRouteInjection_NoUserInContext(t *testing.T) {
	// A proxy with no authorizer or credentials — neither is reached because the
	// secretGetter closure fails first when there is no user in the request context.
	p := &proxy{}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/path", nil)
	route := &mgmt.ProxyEndpointRoute{
		CredentialInjection: &mgmt.CredentialInjectionSpec{Mode: "bearer", TokenField: "token"},
	}

	// "inject credID=..." so getRequestParams parses credID correctly.
	err := p.applyRouteInjection(req, "inject credID=cattle-global-data/my-cred", route)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

// --- perRouteTLSTransport ---

func TestPerRouteTLSTransport_UsesInsecureTransportWhenFlagSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	sentinel := errors.New("insecure transport called")
	insecure := &recordingTransport{err: sentinel}

	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute("api.example.com", true, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache, insecureTransport: insecure}
	transport := &perRouteTLSTransport{proxy: p}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/path", nil)
	_, err := transport.RoundTrip(req)

	assert.ErrorIs(t, err, sentinel, "insecure transport should have been called")
	assert.True(t, insecure.called)
}

func TestPerRouteTLSTransport_UsesDefaultTransportWhenInsecureNotSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	insecure := &recordingTransport{}

	// Use a real test server so the default transport can connect successfully.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	host := ts.Listener.Addr().String() // e.g. "127.0.0.1:PORT"

	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute(host, false, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache, insecureTransport: insecure}
	transport := &perRouteTLSTransport{proxy: p}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/path", nil)
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, insecure.called, "insecure transport must not be used when InsecureSkipTLSVerify is false")
}

func TestPerRouteTLSTransport_UsesDefaultTransportWhenNoRouteMatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	insecure := &recordingTransport{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cache := fake.NewMockNonNamespacedCacheInterface[*mgmt.ProxyEndpoint](ctrl)
	// Return an endpoint that does NOT match the test server's host.
	cache.EXPECT().List(gomock.Any()).Return([]*mgmt.ProxyEndpoint{
		endpointWithRoute("unrelated.domain.com", true, nil),
	}, nil)

	p := &proxy{proxyEndpointCache: cache, insecureTransport: insecure}
	transport := &perRouteTLSTransport{proxy: p}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/path", nil)
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, insecure.called, "insecure transport must not be used when no route matches")
}
