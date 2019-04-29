package httpproxy

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

// ReplaceSetCookies should rename set cookie header to api set cookie header
func TestReplaceSetCookies(t *testing.T) {
	DummyRequest := &http.Response{
		Header: map[string][]string{
			SetCookie:    {"test1=abc", "test2=def", "test3=ghi"},
			APISetCookie: {},
		},
	}

	replaceSetCookies(DummyRequest)
	assert.Equal(t, []string{"test1=abc", "test2=def", "test3=ghi"}, DummyRequest.Header[APISetCookie])
	assert.Equal(t, 0, len(DummyRequest.Header[SetCookie]))

	DummyRequest = &http.Response{
		Header: map[string][]string{
			SetCookie:    {"test1=abc", "test2=def", "test3=ghi"},
			APISetCookie: {"test4=asdf"},
		},
	}

	replaceSetCookies(DummyRequest)
	// Should delete original api set cookie
	assert.Equal(t, []string{"test1=abc", "test2=def", "test3=ghi"}, DummyRequest.Header[APISetCookie])
	assert.Equal(t, 0, len(DummyRequest.Header[SetCookie]))
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
}
