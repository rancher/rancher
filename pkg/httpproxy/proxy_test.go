package httpproxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ReplaceSetCookies should rename set cookie header to api set cookie header
func TestReplaceSetCookies(t *testing.T) {
	DummyRequest := &http.Response{
		Header: map[string][]string{
			SetCookie:    {"test1=abc", "test2=def", "test3=ghi"},
			APISetCookie: {},
		},
	}

	setModifiedHeaders(DummyRequest)
	assert.Equal(t, []string{"test1=abc", "test2=def", "test3=ghi"}, DummyRequest.Header[APISetCookie])
	assert.Equal(t, 0, len(DummyRequest.Header[SetCookie]))
	assert.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"}, DummyRequest.Header[CSP])
	assert.Equal(t, []string{"nosniff"}, DummyRequest.Header[XContentType])

	DummyRequest = &http.Response{
		Header: map[string][]string{
			SetCookie:    {"test1=abc", "test2=def", "test3=ghi"},
			APISetCookie: {"test4=asdf"},
		},
	}

	setModifiedHeaders(DummyRequest)
	// Should delete original api set cookie
	assert.Equal(t, []string{"test1=abc", "test2=def", "test3=ghi"}, DummyRequest.Header[APISetCookie])
	assert.Equal(t, 0, len(DummyRequest.Header[SetCookie]))
	assert.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"}, DummyRequest.Header[CSP])
	assert.Equal(t, []string{"nosniff"}, DummyRequest.Header[XContentType])
}

// ReplaceCookie should delete  current cookie and replace it with api cookie if available
func TestReplaceCookie(t *testing.T) {
	DummyRequest := &http.Request{
		Header: map[string][]string{
			"Cookie": {"abcdef"},
		},
	}

	replaceCookies(DummyRequest)
	assert.Equal(t, "", DummyRequest.Header.Get(Cookie))
	assert.Equal(t, 0, len(DummyRequest.Header[Cookie]))

	DummyRequest = &http.Request{
		Header: map[string][]string{
			Cookie:    {},
			APICookie: {"test1"},
		},
	}

	replaceCookies(DummyRequest)
	assert.Equal(t, "test1", DummyRequest.Header.Get(Cookie))
	assert.Equal(t, "", DummyRequest.Header.Get(APICookie))
	assert.Equal(t, 1, len(DummyRequest.Header[Cookie]))
	assert.Equal(t, 0, len(DummyRequest.Header[APICookie]))

	DummyRequest = &http.Request{
		Header: map[string][]string{
			Cookie:    {},
			APICookie: {"test1", "test2", "test3"},
		},
	}

	replaceCookies(DummyRequest)
	// Should not support multiple cookie headers
	assert.Equal(t, "test1", DummyRequest.Header.Get(Cookie))
	assert.Equal(t, "", DummyRequest.Header.Get(APICookie))
	assert.Equal(t, 1, len(DummyRequest.Header[Cookie]))
	assert.Equal(t, 0, len(DummyRequest.Header[APICookie]))

	DummyRequest = &http.Request{
		Header: map[string][]string{
			Cookie:    {"test0"},
			APICookie: {"test1", "test2", "test3"},
		},
	}

	replaceCookies(DummyRequest)
	// Original cookie should be overwritten
	assert.Equal(t, "test1", DummyRequest.Header.Get(Cookie))
	assert.Equal(t, "", DummyRequest.Header.Get(APICookie))
	assert.Equal(t, 1, len(DummyRequest.Header[Cookie]))
	assert.Equal(t, 0, len(DummyRequest.Header[APICookie]))

	DummyRequest = &http.Request{
		Header: map[string][]string{
			Cookie:    {"test0", "test1"},
			APICookie: {"test2", "test3", "test4"},
		},
	}

	replaceCookies(DummyRequest)
	// Should delete all original cookies
	assert.Equal(t, "test2", DummyRequest.Header.Get(Cookie))
	assert.Equal(t, "", DummyRequest.Header.Get(APICookie))
	assert.Equal(t, 1, len(DummyRequest.Header[Cookie]))
	assert.Equal(t, 0, len(DummyRequest.Header[APICookie]))

}

// IsAllowed should return false if exact domain is a valid host or suffix of host matches wildcard valid host
func TestIsAllowed(t *testing.T) {
	dummyProxy := &proxy{
		validHostsSupplier: func() []string {
			return []string{"test1.com", "test2.io", "test3.org"}
		},
	}

	assert.Equal(t, false, dummyProxy.isAllowed(""))
	assert.Equal(t, false, dummyProxy.isAllowed("test1.org"))
	assert.Equal(t, false, dummyProxy.isAllowed("test4.com"))
	assert.Equal(t, true, dummyProxy.isAllowed("test2.io"))

	dummyProxy = &proxy{
		validHostsSupplier: func() []string {
			return []string{"*test1.com", "test2.io", "test3.org"}
		},
	}

	assert.Equal(t, true, dummyProxy.isAllowed("123test1.com"))
	assert.Equal(t, false, dummyProxy.isAllowed("123test1.io"))

	dummyProxy = &proxy{
		validHostsSupplier: func() []string {
			return []string{"foo.%.alpha.com", "test2.io", "test3.org"}
		},
	}

	assert.Equal(t, false, dummyProxy.isAllowed("123test1.com"))
	assert.Equal(t, true, dummyProxy.isAllowed("foo.bar.alpha.com"))
	assert.Equal(t, false, dummyProxy.isAllowed("foo.bar.baz.alpha.com"))
}

func TestConstructRegex(t *testing.T) {
	type test struct {
		host          string
		whitelistItem string
		found         bool
		description   string
	}

	tests := []test{
		{
			host:          "ec2.us-west-1.amazonaws.com",
			whitelistItem: "ec2.%.amazonaws.com",
			found:         true,
			description:   "base case",
		},
		{
			host:          "ec2.amazonaws.com",
			whitelistItem: "%.amazonaws.com",
			found:         true,
			description:   "base case",
		},
		{
			host:          "foo.ec2.amazonaws.com",
			whitelistItem: "%.amazonaws.com",
			found:         false,
			description:   "base case",
		},
		{
			host:          "ec2.us-west-1.thing.foo.amazonaws.com",
			whitelistItem: "ec2.%.amazonaws.com",
			found:         false,
			description:   "addition content in the middle",
		},
		{
			host:          "thing.us-west-1.amazonaws.com",
			whitelistItem: "ec2.%.amazonaws.com",
			found:         false,
			description:   "should not match prefix",
		},
		{
			host:          "ec2.us-west-1.amazonaws.com.cn",
			whitelistItem: "ec2.%.amazonaws.com",
			found:         false,
			description:   "should not match suffix",
		},
		{
			host:          "iam.cn-north-1.amazonaws.com.cn",
			whitelistItem: "iam.%.amazonaws.com.cn",
			found:         true,
			description:   "base case",
		},
		{
			host:          "iam.cn-north-1.thing.foo.amazonaws.com.cn",
			whitelistItem: "iam.%.amazonaws.com.cn",
			found:         false,
			description:   "addition content in the middle",
		},
		{
			host:          "thing.iam.cn-north-1.amazonaws.com.cn",
			whitelistItem: "iam.%.amazonaws.com.cn",
			found:         false,
			description:   "should not match prefix",
		},
		{
			host:          "iam.cn-north-1.amazonaws.com",
			whitelistItem: "iam.%.amazonaws.com.cn",
			found:         false,
			description:   "should not match suffix",
		},
		{
			host:          "ec2.amazonaws.com",
			whitelistItem: "%c2.amazonaws.com",
			found:         false,
			description:   "must be a complete label",
		},
	}

	for _, scenario := range tests {
		r := constructRegex(scenario.whitelistItem)
		match := r.MatchString(scenario.host)
		assert.Equal(t, scenario.found, match,
			fmt.Sprintf("failed on host %v and whitelist item %v, %v",
				scenario.host,
				scenario.whitelistItem,
				scenario.description,
			))
	}
}

func TestIsOverlyBroad(t *testing.T) {
	tests := []struct {
		name          string
		domain        string
		isOverlyBroad bool
	}{
		{
			name:          "simple url",
			domain:        "example.com",
			isOverlyBroad: false,
		},
		{
			name:          "simple properly scoped wildcard using *",
			domain:        "*.example.com",
			isOverlyBroad: false,
		},
		{
			name:          "simple properly scoped wildcard using %",
			domain:        "%.example.org",
			isOverlyBroad: false,
		},
		{
			name:          "properly scoped wildcard using *",
			domain:        "*example.com",
			isOverlyBroad: false,
		},
		{
			name:          "properly scoped wildcard subdomain using *",
			domain:        "*.sub.example.com",
			isOverlyBroad: false,
		},
		{
			name:          "properly scoped wildcard subdomain using %",
			domain:        "%.sub.example.com",
			isOverlyBroad: false,
		},
		{
			name:          "properly scoped wildcard subdomain using % with multi-part TLD",
			domain:        "%.sub.example.co.uk",
			isOverlyBroad: false,
		},
		{
			name:          "properly scoped wildcard subdomain using * with multi-part TLD",
			domain:        "*.sub.example.co.uk",
			isOverlyBroad: false,
		},
		{
			name:          "overly broad wildcard using *",
			domain:        "*.com",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad wildcard using %",
			domain:        "%.com",
			isOverlyBroad: true,
		},
		{
			name:          "standard multi-part TLD without wildcard, non-ICANN",
			domain:        "objects.rma.cloudscale.ch",
			isOverlyBroad: false,
		},
		{
			name:          "overly broad wildcard using * and a multi-part TLD",
			domain:        "*.co.uk",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad wildcard using % and a multi-part TLD",
			domain:        "%.co.uk",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad wildcard using % and *",
			domain:        "*.%.com",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad wildcard using multiple %",
			domain:        "%.%.org",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad wildcard using multiple % and multi-part TLD",
			domain:        "%.%.co.uk",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad wildcard using % and * and a multi-part TLD",
			domain:        "*.%.gov.uk",
			isOverlyBroad: true,
		},
		{
			name:          "just a TLD",
			domain:        "com",
			isOverlyBroad: false,
		},
		{
			name:          "multi-part TLD only",
			domain:        "co.uk",
			isOverlyBroad: false,
		},
		{
			name:          "overly broad with only star",
			domain:        "*",
			isOverlyBroad: true,
		},
		{
			name:          "overly broad with only percentage",
			domain:        "%",
			isOverlyBroad: true,
		},
		{
			name:          "aws iam.amazonaws.com",
			domain:        "iam.amazonaws.com",
			isOverlyBroad: false,
		},
		{
			name:          "aws iam.us-gov.amazonaws.com",
			domain:        "iam.us-gov.amazonaws.com",
			isOverlyBroad: false,
		},
		{
			name:          "aws iam with % and multi-part TLD",
			domain:        "iam.%.amazonaws.com.cn",
			isOverlyBroad: false,
		},
		{
			name:          "aws iam.global.api.aws",
			domain:        "iam.global.api.aws",
			isOverlyBroad: false,
		},
		{
			name:          "aws ec2 with % (single-part TLD)",
			domain:        "ec2.%.amazonaws.com",
			isOverlyBroad: false,
		},
		{
			name:          "aws ec2 with % and multi-part TLD",
			domain:        "ec2.%.amazonaws.com.cn",
			isOverlyBroad: false,
		},
		{
			name:          "aws ec2 with % and api.aws",
			domain:        "ec2.%.api.aws",
			isOverlyBroad: false,
		},
		{
			name:          "aws eks with %",
			domain:        "eks.%.amazonaws.com",
			isOverlyBroad: false,
		},
		{
			name:          "aws eks with % and multi-part TLD",
			domain:        "eks.%.amazonaws.com.cn",
			isOverlyBroad: false,
		},
		{
			name:          "aws eks with % and api.aws",
			domain:        "eks.%.api.aws",
			isOverlyBroad: false,
		},
		{
			name:          "aws kms with %",
			domain:        "kms.%.amazonaws.com",
			isOverlyBroad: false,
		},
		{
			name:          "aws kms with % and multi-part TLD",
			domain:        "kms.%.amazonaws.com.cn",
			isOverlyBroad: false,
		},
		{
			name:          "aws kms with % and api.aws",
			domain:        "kms.%.api.aws",
			isOverlyBroad: false,
		},
		{
			name:          "cloud.ca objects-east",
			domain:        "objects-east.cloud.ca",
			isOverlyBroad: false,
		},
		{
			name:          "cloudscale objects.rma",
			domain:        "objects.rma.cloudscale.ch",
			isOverlyBroad: false,
		},
		{
			name:          "digitalocean api",
			domain:        "api.digitalocean.com",
			isOverlyBroad: false,
		},
		{
			name:          "exoscale api",
			domain:        "api.exoscale.ch",
			isOverlyBroad: false,
		},
		{
			name:          "linode api",
			domain:        "api.linode.com",
			isOverlyBroad: false,
		},
		{
			name:          "oracle cloud wildcard",
			domain:        "*.oraclecloud.com",
			isOverlyBroad: false,
		},
		{
			name:          "otc wildcard",
			domain:        "*.otc.t-systems.com",
			isOverlyBroad: false,
		},
		{
			name:          "packet api",
			domain:        "api.packet.net",
			isOverlyBroad: false,
		},
		{
			name:          "equinix api",
			domain:        "api.equinix.com",
			isOverlyBroad: false,
		},
		{
			name:          "equinix rancher-drivers",
			domain:        "rancher-drivers.equinixmetal.net",
			isOverlyBroad: false,
		},
		{
			name:          "phoenixnap securedservers api",
			domain:        "api.securedservers.com",
			isOverlyBroad: false,
		},
		{
			name:          "phoenixnap api",
			domain:        "api.phoenixnap.com",
			isOverlyBroad: false,
		},
		{
			name:          "phoenixnap auth",
			domain:        "auth.phoenixnap.com",
			isOverlyBroad: false,
		},

		{
			name:          "nutanix github.io",
			domain:        "nutanix.github.io",
			isOverlyBroad: false,
		},

		{
			name:          "outscale oos",
			domain:        "oos.eu-west-2.outscale.com",
			isOverlyBroad: false,
		},
		{
			name:          "github.com",
			domain:        "github.com",
			isOverlyBroad: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equalf(t, test.isOverlyBroad, isOverlyBroad(test.domain), "failed on domain %v, expected isOverlyBroad to be %v", test.domain, test.isOverlyBroad)
		})
	}
}

func TestIsBadHeader(t *testing.T) {
	tests := []struct {
		key   string
		isBad bool
	}{
		{"X-Forwarded-Proto", true},
		{"Accept-Language", false},
		{"Accept", false},
		{"impersonate-user", true},
		{"impersonate-group", true},
		{"Impersonate-Extra-requesthost", true},
		{"Impersonate-Extra-username", true},
		{"Impersonate-Extra-requesttokenid", true},
		{"Impersonate-Extra-foo", true},
		{"Host", true},
		{"transfer-encoding", true},
		{"Content-Length", true},
		{"X-API-Auth-Header", true},
		{"X-API-CattleAuth-Header", true},
		{"Authorization", true},
		{"authorization", true},
		{"Cookie", true},
		{"cookie", true},
		{"CF-Connecting-IP", true},
		{"CF-Ray", true},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			assert.Equal(t, test.isBad, isBadHeader(test.key))
		})
	}
}

// ReverseProxy merges the upstream headers in with Add, so without stripInheritedCSP the client receives two Content-Security-Policy headers.
func TestInheritedCSPIsNotDuplicated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	assert.NoError(t, err)

	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = upstreamURL.Scheme
			req.URL.Host = upstreamURL.Host
			req.Host = upstreamURL.Host
		},
		ModifyResponse: setModifiedHeaders,
	}

	// globalMiddleware stands in for the chain in pkg/rancher/rancher.go, which sets the header before the request reaches here.
	globalMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.Header().Set(CSP, "base-uri 'self'; object-src 'none'; frame-ancestors 'self'")
			next.ServeHTTP(rw, req)
		})
	}

	recorder := httptest.NewRecorder()
	globalMiddleware(stripInheritedCSP(reverseProxy)).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/meta/proxy/example.com/", nil))

	// The proxy's own sandboxing policy must be the only one sent.
	assert.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"}, recorder.Header()[CSP])
}

// TestProxyOutgoingHeaders checks the outgoing Cookie and Authorization
// headers for each authentication path.
func TestProxyOutgoingHeaders(t *testing.T) {
	p := &proxy{
		prefix: "/proxy/",
		validHostsSupplier: func() []string {
			return []string{"api.example.com"}
		},
	}
	tests := []struct {
		name          string
		headers       map[string]string
		wantErr       bool
		wantAuth      string // "" = no Authorization header at all
		wantExtraName string
		wantExtraVal  string
	}{
		{
			name:    "no auth headers",
			headers: map[string]string{},
		},
		{
			name:     "X-API-Auth-Header",
			headers:  map[string]string{APIAuth: "Bearer plain"},
			wantAuth: "Bearer plain",
		},
		{
			name:     "CattleAuth without a known signer",
			headers:  map[string]string{CattleAuth: "token abc"},
			wantAuth: "token abc",
		},
		{
			name:          "arbitrary signer",
			headers:       map[string]string{CattleAuth: "arbitrary headers=X-Extra=one"},
			wantExtraName: "X-Extra",
			wantExtraVal:  "one",
		},
		{
			name:    "bearer signer error",
			headers: map[string]string{CattleAuth: "bearer credID=cattle-global-data:cc-x passwordField=accessToken"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://rancher.example.com/meta/proxy/api.example.com/v2/regions", nil)
			req.Header.Set(AuthHeader, "Bearer token-abc:secret")
			req.Header.Set(Cookie, "R_SESS=token-abc:secret; other=1")
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			err := p.proxy(req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Empty(t, req.Header.Values(Cookie))
			if tt.wantAuth == "" {
				assert.Empty(t, req.Header.Values(AuthHeader))
			} else {
				assert.Equal(t, []string{tt.wantAuth}, req.Header.Values(AuthHeader))
			}
			if tt.wantExtraName != "" {
				assert.Equal(t, tt.wantExtraVal, req.Header.Get(tt.wantExtraName))
			}
		})
	}
}

// TestProxyAPICookieWithSigner checks X-Api-Cookie-Header becomes the
// outgoing Cookie header when a signer is used.
func TestProxyAPICookieWithSigner(t *testing.T) {
	p := &proxy{
		prefix: "/proxy/",
		validHostsSupplier: func() []string {
			return []string{"api.example.com"}
		},
	}
	req := httptest.NewRequest(http.MethodGet, "https://rancher.example.com/meta/proxy/api.example.com/v2/regions", nil)
	req.Header.Set(Cookie, "R_SESS=token-abc:secret")
	req.Header.Set(APICookie, "mine=1")
	req.Header.Set(CattleAuth, "arbitrary headers=X-Extra=one")
	assert.NoError(t, p.proxy(req))
	assert.Equal(t, []string{"mine=1"}, req.Header.Values(Cookie))
	assert.Empty(t, req.Header.Values(APICookie))
}

type recordedRequest struct {
	method string
	path   string
	cookie string
}

type upstreamRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

func (u *upstreamRecorder) record(req *http.Request) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.requests = append(u.requests, recordedRequest{
		method: req.Method,
		path:   req.URL.Path,
		cookie: req.Header.Get(Cookie),
	})
}

func (u *upstreamRecorder) all() []recordedRequest {
	u.mu.Lock()
	defer u.mu.Unlock()

	return slices.Clone(u.requests)
}

// startUpstream starts the host the proxy is asked to reach.
func startUpstream(t *testing.T, body string) (*upstreamRecorder, string) {
	t.Helper()

	recorder := &upstreamRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recorder.record(req)
		fmt.Fprint(rw, body)
	}))
	t.Cleanup(srv.Close)

	return recorder, strings.TrimPrefix(srv.URL, "http://")
}

// startMetaProxy mounts the proxy at /meta/proxy/, matching how routes.go
// registers it, and returns the address to send requests to.
func startMetaProxy(t *testing.T, validHosts []string) string {
	t.Helper()

	handler := newHandler(&proxy{
		prefix:             "/proxy/",
		validHostsSupplier: func() []string { return validHosts },
	})

	mux := http.NewServeMux()
	mux.Handle("/meta/proxy/", handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return strings.TrimPrefix(srv.URL, "http://")
}

// rawRequest writes requestLine verbatim so the test controls the request-target
// form. An http.Client cannot do this: it only emits an absolute-form target when
// it believes it is talking to a configured proxy.
func rawRequest(t *testing.T, addr, requestLine string) *http.Response {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(30*time.Second)))

	_, err = fmt.Fprintf(conn,
		"%s HTTP/1.1\r\nHost: rancher.example.com\r\nCookie: R_SESS=session-token\r\nConnection: close\r\n\r\n",
		requestLine)
	require.NoError(t, err)

	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	t.Cleanup(func() { res.Body.Close() })

	return res
}

func TestProxyRequestTargetHandling(t *testing.T) {
	t.Parallel()

	const upstreamBody = "upstream-response"

	tests := []struct {
		name string
		// validHosts is the allowlist. The upstream listens on 127.0.0.1, so
		// "127.0.0.1" permits it and anything else rejects it.
		validHosts   []string
		requestLine  func(upstreamAddr, proxyAddr string) string
		wantStatus   int
		wantUpstream []recordedRequest
	}{
		{
			name:       "allowed host",
			validHosts: []string{"127.0.0.1"},
			requestLine: func(upstreamAddr, _ string) string {
				return fmt.Sprintf("GET /meta/proxy/http:/%s/data", upstreamAddr)
			},
			wantStatus:   http.StatusOK,
			wantUpstream: []recordedRequest{{method: http.MethodGet, path: "/data"}},
		},
		{
			name:       "allowed host, absolute-form request target",
			validHosts: []string{"127.0.0.1"},
			requestLine: func(upstreamAddr, proxyAddr string) string {
				return fmt.Sprintf("GET http://%s/meta/proxy/http:/%s/data", proxyAddr, upstreamAddr)
			},
			wantStatus:   http.StatusOK,
			wantUpstream: []recordedRequest{{method: http.MethodGet, path: "/data"}},
		},
		{
			name:       "host not allowed",
			validHosts: []string{"allowed.example.com"},
			requestLine: func(upstreamAddr, _ string) string {
				return fmt.Sprintf("GET /meta/proxy/http:/%s/data", upstreamAddr)
			},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "absolute-form request target",
			validHosts: []string{"allowed.example.com"},
			requestLine: func(upstreamAddr, _ string) string {
				return fmt.Sprintf("GET http://%s/meta/proxy/", upstreamAddr)
			},
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "absolute-form request target with encoded path",
			validHosts: []string{"allowed.example.com"},
			requestLine: func(upstreamAddr, _ string) string {
				return fmt.Sprintf("PUT http://%s/meta/proxy/%%2e%%2e/%%2e%%2e/data", upstreamAddr)
			},
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			upstream, upstreamAddr := startUpstream(t, upstreamBody)
			proxyAddr := startMetaProxy(t, test.validHosts)

			res := rawRequest(t, proxyAddr, test.requestLine(upstreamAddr, proxyAddr))
			assert.Equal(t, test.wantStatus, res.StatusCode)

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			if test.wantStatus == http.StatusOK {
				assert.Equal(t, upstreamBody, string(body))
			} else {
				assert.NotContains(t, string(body), upstreamBody)
			}

			assert.Equal(t, test.wantUpstream, upstream.all())
		})
	}
}

func TestProxyForwardedRequestCookie(t *testing.T) {
	t.Parallel()

	upstream, upstreamAddr := startUpstream(t, "upstream-response")
	proxyAddr := startMetaProxy(t, []string{"127.0.0.1"})

	res := rawRequest(t, proxyAddr, fmt.Sprintf("GET /meta/proxy/http:/%s/data", upstreamAddr))
	require.Equal(t, http.StatusOK, res.StatusCode)

	requests := upstream.all()
	require.Len(t, requests, 1)
	assert.Empty(t, requests[0].cookie)
}
