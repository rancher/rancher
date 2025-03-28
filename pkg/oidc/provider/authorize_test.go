package provider

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/oidc/provider/session"
	"github.com/rancher/rancher/pkg/settings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAuthEndpoint(t *testing.T) {
	const (
		fakeTokenName   = "fake-token-name"
		fakeTokenValue  = "fake-token-value"
		fakeUserID      = "fake-user-id"
		fakeCode        = "fake-code"
		fakeRedirectUri = "https://www.rancher.com"
		fakeClientID    = "client-id"
		fakeClientName  = "client-name"
		fakeServerUrl   = "https://www.fake.com"
	)
	type mockParams struct {
		tokenCache      *fake.MockNonNamespacedCacheInterface[*v3.Token]
		userLister      *fake.MockNonNamespacedCacheInterface[*v3.User]
		oidcClientCache *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient]
		codeCreator     *mocks.MockcodeCreator
		sessionAdder    *mocks.MocksessionAdder
	}
	_ = settings.ServerURL.Set(fakeServerUrl)
	fakeTime := time.Unix(0, 0)
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		req          func() *http.Request
		mockSetup    func(mockParams)
		wantRedirect string
		wantHttpCode int
		wantError    string
	}{
		"redirect with code when Rancher token in present": {
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.sessionAdder.EXPECT().Add(fakeCode, session.Session{
					ClientID:      fakeClientID,
					TokenName:     fakeTokenName,
					Scope:         []string{"openid"},
					CodeChallenge: "code-challenge",
					CreatedAt:     fakeTime,
				})
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
				m.codeCreator.EXPECT().GenerateCode().Return(fakeCode, nil)
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?code=fake-code",
		},
		"redirect to login page if Rancher token is not present": {
			req: func() *http.Request {
				return &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&code_challenge=code-challenge&response_type=code&redirect_uri=https://www.rancher.com&scope=openid&client_id=client-id",
					},
					Method: http.MethodGet,
				}
			},
			mockSetup: func(m mockParams) {
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeServerUrl + "/dashboard/auth/login?client_id=client-id&code_challenge=code-challenge&redirect_uri=https%3A%2F%2Fwww.rancher.com&response_type=code&scope=openid",
		},
		"response type not supported": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=none&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=unsupported_response_type&error_description=response+type+not+supported",
		},
		"code challenge method not supported": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=plain&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=invalid_request&error_description=challenge_method+not+supported%2C+only+S256+is+supported",
		},
		"missing openid": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=profile&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=invalid_scope&error_description=missing+openid+scope",
		},
		"invalid scope": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid+invalidscope&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=invalid_scope&error_description=invalid+scope%3A+invalidscope",
		},
		"missing code challenge": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=invalid_request&error_description=missing+code_challenge",
		},
		"missing redirect uri": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid",
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusBadRequest,
			wantError:    `{"error":"invalid_request","error_description":"missing redirect_uri"}`,
		},
		"oidc client not registered": {
			mockSetup: func(m mockParams) {
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return(nil, errors.NewNotFound(schema.GroupResource{}, fakeClientID))
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=server_error&error_description=error+retrieving+OIDC+client%3A++%22client-id%22+not+found",
		},
		"redirect uri not registed": {
			mockSetup: func(m mockParams) {
				m.oidcClientCache.EXPECT().GetByIndex("oidc.management.cattle.io/oidcclient-by-id", fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{"anotherurl"},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeRedirectUri + "?error=invalid_request&error_description=redirect_uri+https%3A%2F%2Fwww.rancher.com+is+not+registered",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := mockParams{
				tokenCache:      fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl),
				userLister:      fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl),
				oidcClientCache: fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl),
				sessionAdder:    mocks.NewMocksessionAdder(ctrl),
				codeCreator:     mocks.NewMockcodeCreator(ctrl),
			}
			if test.mockSetup != nil {
				test.mockSetup(m)
			}
			h := newAuthorizeHandler(m.tokenCache, m.userLister, m.sessionAdder, m.codeCreator, m.oidcClientCache)
			h.now = func() time.Time {
				return fakeTime
			}
			rec := httptest.NewRecorder()

			h.authEndpoint(rec, test.req())

			assert.Equal(t, test.wantHttpCode, rec.Code)
			if test.wantRedirect != "" {
				assert.Equal(t, test.wantRedirect, rec.Header().Get("Location"))
			}
			if test.wantError != "" {
				assert.JSONEq(t, test.wantError, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}
