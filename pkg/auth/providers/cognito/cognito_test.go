package cognito

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	logoutPath    = "/v3/tokens?action=logout"
	logoutAllPath = "/v3/tokens?action=logoutAll"
)

func TestLogoutAllWhenNotEnabled(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "cognito"
	)

	oidcConfig := newOIDCConfig("8090", func(s *v3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
		s.LogoutAllEnabled = false
	})
	testToken := &v3.Token{UserID: userId, AuthProvider: providerName}
	o := CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			OpenIDCProvider: oidc.OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*v3.OIDCConfig, error) { return oidcConfig, nil },
			},
		},
	}

	b, err := json.Marshal(&v3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, logoutAllPath, bytes.NewReader(b))
	w := httptest.NewRecorder()

	assert.ErrorContains(t, o.LogoutAll(w, r, testToken), "Rancher provider resource `cognito` not configured for SLO")
}

func TestLogoutAll(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "cognito"
	)

	oidcConfig := newOIDCConfig("8090", func(s *v3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
	})
	testToken := &v3.Token{UserID: userId, AuthProvider: providerName}
	o := CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			OpenIDCProvider: oidc.OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*v3.OIDCConfig, error) { return oidcConfig, nil },
			},
		},
	}
	b, err := json.Marshal(&v3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, logoutAllPath, bytes.NewReader(b))
	w := httptest.NewRecorder()

	require.NoError(t, o.LogoutAll(w, r, testToken))

	require.Equal(t, http.StatusOK, w.Code)
	wantData := map[string]any{
		"idpRedirectUrl": "http://localhost:8090/user/logout?client_id=test&logout_uri=https%3A%2F%2Fexample.com%2Flogged-out",
		"type":           "authConfigLogoutOutput",
		"baseType":       "authConfigLogoutOutput",
	}
	gotData := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotData))
	assert.Equal(t, wantData, gotData)
}

func TestLogoutAllNoEndSessionEndpoint(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "oidc"
	)

	oidcConfig := newOIDCConfig("8090")
	testToken := &v3.Token{UserID: userId, AuthProvider: providerName}
	o := CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			OpenIDCProvider: oidc.OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*v3.OIDCConfig, error) { return oidcConfig, nil },
			},
		},
	}
	b, err := json.Marshal(&v3.AuthConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, logoutAllPath, bytes.NewReader(b))
	w := httptest.NewRecorder()

	assert.ErrorContains(t, o.LogoutAll(w, r, testToken), "LogoutAll triggered with no endSessionEndpoint")
}

func TestLogout(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "cognito"
	)

	logoutTests := map[string]struct {
		config *v3.OIDCConfig
		verify func(t require.TestingT, err error, msgAndArgs ...any)
	}{
		"when logout all is forced": {
			config: newOIDCConfig("9090", func(s *v3.OIDCConfig) {
				s.LogoutAllForced = true
			}),
			verify: require.Error,
		},
		"when logout all is not forced": {
			config: newOIDCConfig("9090"),
			verify: require.NoError,
		},
	}

	for name, tt := range logoutTests {
		t.Run(name, func(t *testing.T) {
			testToken := &v3.Token{UserID: userId, AuthProvider: providerName}
			o := CognitoProvider{
				GenOIDCProvider: genericoidc.GenOIDCProvider{
					OpenIDCProvider: oidc.OpenIDCProvider{
						Name:      providerName,
						GetConfig: func() (*v3.OIDCConfig, error) { return tt.config, nil },
					},
				},
			}

			b, err := json.Marshal(&v3.AuthConfigLogoutInput{
				FinalRedirectURL: "https://example.com/logged-out",
			})
			require.NoError(t, err)

			r := httptest.NewRequest(http.MethodPost, logoutPath, bytes.NewReader(b))
			w := httptest.NewRecorder()

			tt.verify(t, o.Logout(w, r, testToken))
		})
	}
}

func newOIDCConfig(port string, opts ...func(*v3.OIDCConfig)) *v3.OIDCConfig {
	cfg := &v3.OIDCConfig{
		Issuer:           "http://localhost:" + port,
		ClientID:         "test",
		JWKSUrl:          "http://localhost:" + port + "/.well-known/jwks.json",
		AuthEndpoint:     "http://localhost:" + port + "/auth",
		TokenEndpoint:    "http://localhost:" + port + "/token",
		UserInfoEndpoint: "http://localhost:" + port + "/user",
		LogoutAllEnabled: true,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}
