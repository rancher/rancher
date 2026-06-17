package saml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/golang-jwt/jwt/v5"
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
				serviceProvider: &saml.ServiceProvider{
					Key: privateKey,
				},
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

	tests := map[string]struct {
		conditions *saml.Conditions
		wantErr    bool
	}{
		"nil conditions": {
			conditions: nil,
			wantErr:    false,
		},
		"valid window: NotBefore in past, NotOnOrAfter in future": {
			conditions: &saml.Conditions{NotBefore: past, NotOnOrAfter: future},
			wantErr:    false,
		},
		"NotBefore in the future": {
			conditions: &saml.Conditions{NotBefore: future},
			wantErr:    true,
		},
		"NotOnOrAfter in the past": {
			conditions: &saml.Conditions{NotOnOrAfter: past},
			wantErr:    true,
		},
		"NotOnOrAfter exactly equal to now (on or after boundary)": {
			conditions: &saml.Conditions{NotOnOrAfter: now},
			wantErr:    true,
		},
		"NotBefore exactly equal to now (valid: now is not before itself)": {
			conditions: &saml.Conditions{NotBefore: now},
			wantErr:    false,
		},
		"only NotBefore set, in the past": {
			conditions: &saml.Conditions{NotBefore: past},
			wantErr:    false,
		},
		"only NotOnOrAfter set, in the future": {
			conditions: &saml.Conditions{NotOnOrAfter: future},
			wantErr:    false,
		},
		"zero-value Conditions (both bounds unset)": {
			conditions: &saml.Conditions{},
			wantErr:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := checkAssertionTimeConditions(now, tc.conditions)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
