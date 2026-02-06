package httpproxy

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
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
		{"CF-Connecting-IP", true},
		{"CF-Ray", true},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			assert.Equal(t, test.isBad, isBadHeader(test.key))
		})
	}
}
