package saml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/assert"
)

func TestGetUserIdFromRelayState(t *testing.T) {
	host := "http://www.rancher.com/"
	relayStateValue := "mockValue"
	mockUserID := "u-neuwrd"
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	tests := map[string]struct {
		createRequest func() *http.Request
		wantUserID    string
		wantErr       string
	}{
		"valid userId": {
			createRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, host, nil)
				req.Form = map[string][]string{
					"RelayState": {relayStateValue},
				}
				assert.NoError(t, req.ParseForm())

				secretBlock := x509.MarshalPKCS1PrivateKey(privateKey)
				state := jwt.New(jwt.SigningMethodHS256)
				claims := state.Claims.(jwt.MapClaims)
				claims[rancherUserID] = mockUserID

				signedState, err := state.SignedString(secretBlock)
				assert.NoError(t, err)

				req.Header = map[string][]string{
					"Cookie": {"saml_Rancher_FinalRedirectURL=redirectURL;saml_" + relayStateValue + "=" + signedState},
				}

				return req
			},
			wantUserID: mockUserID,
		},
		"userId not present": {
			createRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, host, nil)
				req.Form = map[string][]string{
					"RelayState": {relayStateValue},
				}
				assert.NoError(t, req.ParseForm())

				secretBlock := x509.MarshalPKCS1PrivateKey(privateKey)
				state := jwt.New(jwt.SigningMethodHS256)
				signedState, err := state.SignedString(secretBlock)
				assert.NoError(t, err)

				req.Header = map[string][]string{
					"Cookie": {"saml_Rancher_FinalRedirectURL=redirectURL;saml_" + relayStateValue + "=" + signedState},
				}

				return req
			},
		},
		"relay state not present": {
			createRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, host, nil)
			},
		},
		"invalid token": {
			createRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, host, nil)
				req.Form = map[string][]string{
					"RelayState": {relayStateValue},
				}
				assert.NoError(t, req.ParseForm())
				req.Header = map[string][]string{
					"Cookie": {"saml_Rancher_FinalRedirectURL=redirectURL;saml_" + relayStateValue + "=wrongToken"},
				}

				return req
			},
			wantErr: "error parsing relay state token",
		},
		"state signed with a different key": {
			createRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, host, nil)
				req.Form = map[string][]string{
					"RelayState": {relayStateValue},
				}
				assert.NoError(t, req.ParseForm())

				anotherKey, err := rsa.GenerateKey(rand.Reader, 2048)
				assert.NoError(t, err)
				secretBlock := x509.MarshalPKCS1PrivateKey(anotherKey)
				state := jwt.New(jwt.SigningMethodHS256)
				claims := state.Claims.(jwt.MapClaims)
				claims[rancherUserID] = mockUserID

				signedState, err := state.SignedString(secretBlock)
				assert.NoError(t, err)

				req.Header = map[string][]string{
					"Cookie": {"saml_Rancher_FinalRedirectURL=redirectURL;saml_" + relayStateValue + "=" + signedState},
				}

				return req
			},
			wantErr: "signature is invalid",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cookieStore := &ClientCookies{
				Name:   "token",
				Domain: host,
			}
			p := Provider{
				clientState: cookieStore,
				spKey:       privateKey,
			}

			userID, err := p.getUserIdFromRelayStateCookie(test.createRequest())
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.wantUserID, userID)
		})
	}
}

func TestValidateFinalRedirectURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		redirectURL string
		rancherURL  string
		want        string
		wantErr     string
	}{
		"empty redirect": {
			redirectURL: "",
			rancherURL:  "https://rancher.example.com",
			wantErr:     "redirect URL was not provided",
		},
		"invalid redirect": {
			redirectURL: "http://[::1",
			rancherURL:  "https://rancher.example.com",
			wantErr:     "invalid redirect URL",
		},
		"invalid rancher url": {
			redirectURL: "https://rancher.example.com/verify",
			rancherURL:  "::://not-a-url",
			wantErr:     "could not parse Rancher server URL",
		},
		"mismatched hosts": {
			redirectURL: "https://attacker.example.com/login",
			rancherURL:  "https://rancher.example.com",
			wantErr:     "does not match Rancher host",
		},
		"matching host": {
			redirectURL: "https://rancher.example.com/dashboard/auth?token=abc",
			rancherURL:  "https://rancher.example.com",
			want:        "https://rancher.example.com/dashboard/auth?token=abc",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := validateFinalRedirectURL(tt.redirectURL, tt.rancherURL)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedirectURLWithError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseURL   string
		errorCode int
		errMsg    string
		wantURL   string
	}{
		"error code only, no existing query params": {
			baseURL:   "https://rancher.example.com/login",
			errorCode: 403,
			errMsg:    "",
			wantURL:   "https://rancher.example.com/login?errorCode=403",
		},
		"error code and message": {
			baseURL:   "https://rancher.example.com/login",
			errorCode: 500,
			errMsg:    "something went wrong",
			wantURL:   "https://rancher.example.com/login?err=something+went+wrong&errorCode=500",
		},
		"preserves existing query params": {
			baseURL:   "https://rancher.example.com/login?token=abc",
			errorCode: 422,
			errMsg:    "",
			wantURL:   "https://rancher.example.com/login?errorCode=422&token=abc",
		},
		"preserves existing query params with message": {
			baseURL:   "https://rancher.example.com/login?token=abc",
			errorCode: 422,
			errMsg:    "user not found",
			wantURL:   "https://rancher.example.com/login?err=user+not+found&errorCode=422&token=abc",
		},
		"error message with special characters is encoded": {
			baseURL:   "https://rancher.example.com/login",
			errorCode: 500,
			errMsg:    "error: bad request & more",
			wantURL:   "https://rancher.example.com/login?err=error%3A+bad+request+%26+more&errorCode=500",
		},
		"403 error code": {
			baseURL:   "https://rancher.example.com/login",
			errorCode: 403,
			errMsg:    "",
			wantURL:   "https://rancher.example.com/login?errorCode=403",
		},
		"unparseable base URL falls back to string concatenation": {
			baseURL:   "http://[::1",
			errorCode: 500,
			errMsg:    "",
			wantURL:   "http://[::1?errorCode=500",
		},
		"unparseable base URL with error message uses QueryEscape": {
			baseURL:   "http://[::1",
			errorCode: 403,
			errMsg:    "access denied",
			wantURL:   "http://[::1?errorCode=403&err=access+denied",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := redirectURLWithError(tt.baseURL, tt.errorCode, tt.errMsg)
			assert.Equal(t, tt.wantURL, got)
		})
	}
}

func TestExtractIDPURLs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		metadata *types.EntityDescriptor
		wantSSO  string
		wantSLO  string
	}{
		"nil IDPSSODescriptor returns empty strings": {
			metadata: &types.EntityDescriptor{IDPSSODescriptor: nil},
			wantSSO:  "",
			wantSLO:  "",
		},
		"empty SSO and SLO service lists return empty strings": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{},
					SingleLogoutServices: []types.SingleLogoutService{},
				},
			},
			wantSSO: "",
			wantSLO: "",
		},
		"SSO URL extracted, no SLO service": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{
						{
							Location: "https://idp.example.com/sso",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
					},
				},
			},
			wantSSO: "https://idp.example.com/sso",
			wantSLO: "",
		},
		"SSO URL extracted - multiple bindings": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{
						{
							Location: "https://idp.example.com/sso",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
						{
							Location: "https://idp.example.com/sso/post",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:POST",
						},
					},
				},
			},
			wantSSO: "https://idp.example.com/sso",
			wantSLO: "",
		},
		"SLO URL extracted, no SSO service": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleLogoutServices: []types.SingleLogoutService{
						{
							Location: "https://idp.example.com/slo",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
					},
				},
			},
			wantSSO: "",
			wantSLO: "https://idp.example.com/slo",
		},
		"both SSO and SLO URLs extracted": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{
						{
							Location: "https://idp.example.com/sso",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
					},
					SingleLogoutServices: []types.SingleLogoutService{
						{
							Location: "https://idp.example.com/slo",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
					},
				},
			},
			wantSSO: "https://idp.example.com/sso",
			wantSLO: "https://idp.example.com/slo",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotSSO, gotSLO := extractIDPURLs(tt.metadata)
			assert.Equal(t, tt.wantSSO, gotSSO)
			assert.Equal(t, tt.wantSLO, gotSLO)
		})
	}
}
