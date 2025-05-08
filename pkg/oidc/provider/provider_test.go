package provider

import (
	"errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddHeadersMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	const fakeRedirectUrl = "http://test.com"
	fakeUrl, _ := url.Parse(fakeRedirectUrl)
	tests := map[string]struct {
		r                *http.Request
		oidcClientCache  func() *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient]
		expectedHeaders  http.Header
		expectedCallNext bool
		expectedBody     string
	}{
		"all headers - request url matches redirect url": {
			r: &http.Request{
				URL: fakeUrl,
			},
			oidcClientCache: func() *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl)
				mock.EXPECT().List(labels.Everything()).Return([]*v3.OIDCClient{
					{
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUrl},
						},
					},
				}, nil)

				return mock
			},
			expectedHeaders: http.Header{
				"X-Content-Type-Options":       []string{"nosniff"},
				"X-Frame-Options":              []string{"SAMEORIGIN"},
				"Referrer-Policy":              []string{"strict-origin-when-cross-origin"},
				"Strict-Transport-Security":    []string{"max-age=31536000"},
				"Access-Control-Allow-Methods": []string{"GET, POST"},
				"Access-Control-Allow-Origin":  []string{fakeRedirectUrl},
			},
			expectedCallNext: true,
		},
		"request url doesn't match redirect url": {
			r: &http.Request{
				URL: fakeUrl,
			},
			oidcClientCache: func() *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl)
				mock.EXPECT().List(labels.Everything()).Return([]*v3.OIDCClient{
					{
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{"www.example.com"},
						},
					},
				}, nil)

				return mock
			},
			expectedHeaders: http.Header{
				"X-Content-Type-Options":       []string{"nosniff"},
				"X-Frame-Options":              []string{"SAMEORIGIN"},
				"Referrer-Policy":              []string{"strict-origin-when-cross-origin"},
				"Strict-Transport-Security":    []string{"max-age=31536000"},
				"Access-Control-Allow-Methods": []string{"GET, POST"},
			},
			expectedCallNext: true,
		},
		"error fetching OIDCClient list": {
			r: &http.Request{
				URL: fakeUrl,
			},
			oidcClientCache: func() *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl)
				mock.EXPECT().List(labels.Everything()).Return(nil, errors.New("unexpected error"))

				return mock
			},
			expectedHeaders: http.Header{
				"X-Content-Type-Options":       []string{"nosniff"},
				"X-Frame-Options":              []string{"SAMEORIGIN"},
				"Referrer-Policy":              []string{"strict-origin-when-cross-origin"},
				"Strict-Transport-Security":    []string{"max-age=31536000"},
				"Access-Control-Allow-Methods": []string{"GET, POST"},
				"Content-Type":                 []string{"application/json"},
			},
			expectedCallNext: false,
			expectedBody:     `{"error":"server_error","error_description":"failed to list OIDCCLients"}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := Provider{
				authHandler: &authorizeHandler{
					oidcClientCache: test.oidcClientCache(),
				},
			}
			rec := httptest.NewRecorder()

			p.addHeadersMiddleware(func(writer http.ResponseWriter, request *http.Request) {
				if !test.expectedCallNext {
					assert.Fail(t, "unexpected call")
				}
			}).ServeHTTP(rec, test.r)

			assert.Equal(t, test.expectedHeaders, rec.Header())
			if test.expectedBody != "" {
				assert.JSONEq(t, test.expectedBody, rec.Body.String())
			}
		})
	}
}
