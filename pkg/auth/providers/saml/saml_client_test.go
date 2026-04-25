package saml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

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
