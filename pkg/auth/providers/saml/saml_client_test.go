package saml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestAssertionCache(t *testing.T) {
	t.Parallel()

	t.Run("new ID is not seen", func(t *testing.T) {
		t.Parallel()
		c := newAssertionCache()
		expiry := time.Now().Add(time.Minute)
		assert.False(t, c.seen("id-1", expiry))
	})

	t.Run("same ID seen twice", func(t *testing.T) {
		t.Parallel()
		c := newAssertionCache()
		expiry := time.Now().Add(time.Minute)
		assert.False(t, c.seen("id-replay", expiry))
		assert.True(t, c.seen("id-replay", expiry))
	})

	t.Run("different IDs are tracked independently", func(t *testing.T) {
		t.Parallel()
		c := newAssertionCache()
		expiry := time.Now().Add(time.Minute)
		assert.False(t, c.seen("id-a", expiry))
		assert.False(t, c.seen("id-b", expiry))
		assert.True(t, c.seen("id-a", expiry))
		assert.True(t, c.seen("id-b", expiry))
	})

	t.Run("expired ID is evicted and accepted again", func(t *testing.T) {
		t.Parallel()
		c := newAssertionCache()
		// Insert with an already-expired time.
		pastExpiry := time.Now().Add(-time.Second)
		assert.False(t, c.seen("id-expired", pastExpiry))

		// The entry is expired; the next call should evict it and accept a fresh one.
		futureExpiry := time.Now().Add(time.Minute)
		assert.False(t, c.seen("id-expired", futureExpiry))
	})

	t.Run("eviction does not remove live entries", func(t *testing.T) {
		t.Parallel()
		c := newAssertionCache()
		liveExpiry := time.Now().Add(time.Minute)
		pastExpiry := time.Now().Add(-time.Second)

		assert.False(t, c.seen("id-live", liveExpiry))
		assert.False(t, c.seen("id-stale", pastExpiry))

		// Trigger eviction via any seen() call.
		assert.False(t, c.seen("id-new", liveExpiry))

		// The live entry must still be detected as a replay.
		assert.True(t, c.seen("id-live", liveExpiry))
	})
}

func TestCheckAssertionTimeConditions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	rfc3339 := func(t time.Time) string {
		return t.Format(time.RFC3339)
	}

	tests := map[string]struct {
		conditions *types.Conditions
		wantErr    bool
	}{
		"nil conditions": {
			conditions: nil,
			wantErr:    false,
		},
		"valid window: NotBefore in past, NotOnOrAfter in future": {
			conditions: &types.Conditions{NotBefore: rfc3339(past), NotOnOrAfter: rfc3339(future)},
			wantErr:    false,
		},
		"NotBefore in the future": {
			conditions: &types.Conditions{NotBefore: rfc3339(future)},
			wantErr:    true,
		},
		"NotOnOrAfter in the past": {
			conditions: &types.Conditions{NotOnOrAfter: rfc3339(past)},
			wantErr:    true,
		},
		"NotOnOrAfter exactly equal to now (on or after boundary)": {
			conditions: &types.Conditions{NotOnOrAfter: rfc3339(now)},
			wantErr:    true,
		},
		"NotBefore exactly equal to now (valid: now is not before itself)": {
			conditions: &types.Conditions{NotBefore: rfc3339(now)},
			wantErr:    false,
		},
		"only NotBefore set, in the past": {
			conditions: &types.Conditions{NotBefore: rfc3339(past)},
			wantErr:    false,
		},
		"only NotOnOrAfter set, in the future": {
			conditions: &types.Conditions{NotOnOrAfter: rfc3339(future)},
			wantErr:    false,
		},
		"zero-value Conditions (both bounds unset)": {
			conditions: &types.Conditions{},
			wantErr:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			notOnOrAfter, err := checkAssertionTimeConditions(now, tc.conditions)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.conditions != nil && tc.conditions.NotOnOrAfter != "" {
					assert.Equal(t, rfc3339(*notOnOrAfter), tc.conditions.NotOnOrAfter)
				}
			}
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
		"SSO falls back to first entry when no HTTP-Redirect binding": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{
						{
							Location: "https://idp.example.com/sso/post",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
						},
						{
							Location: "https://idp.example.com/sso/artifact",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact",
						},
					},
				},
			},
			wantSSO: "https://idp.example.com/sso/post",
			wantSLO: "",
		},
		"SLO falls back to first entry when no HTTP-Redirect binding": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleLogoutServices: []types.SingleLogoutService{
						{
							Location: "https://idp.example.com/slo/post",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
						},
						{
							Location: "https://idp.example.com/slo/artifact",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact",
						},
					},
				},
			},
			wantSSO: "",
			wantSLO: "https://idp.example.com/slo/post",
		},
		"SSO prefers HTTP-Redirect over earlier non-redirect entry": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{
						{
							Location: "https://idp.example.com/sso/post",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
						},
						{
							Location: "https://idp.example.com/sso/redirect",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
					},
				},
			},
			wantSSO: "https://idp.example.com/sso/redirect",
			wantSLO: "",
		},
		"SSO redirect present, SLO falls back to first entry": {
			metadata: &types.EntityDescriptor{
				IDPSSODescriptor: &types.IDPSSODescriptor{
					SingleSignOnServices: []types.SingleSignOnService{
						{
							Location: "https://idp.example.com/sso/redirect",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
						},
					},
					SingleLogoutServices: []types.SingleLogoutService{
						{
							Location: "https://idp.example.com/slo/post",
							Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
						},
					},
				},
			},
			wantSSO: "https://idp.example.com/sso/redirect",
			wantSLO: "https://idp.example.com/slo/post",
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
