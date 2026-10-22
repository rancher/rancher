package httpproxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestIsBadHeader(t *testing.T) {
	tests := []struct {
		key   string
		isBad bool
	}{
		{"X-Forwarded-Proto", false},
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
