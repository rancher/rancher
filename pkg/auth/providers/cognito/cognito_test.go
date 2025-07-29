package cognito

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/stretchr/testify/require"
)

func TestLogoutAll(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "keycloak"
	)
	oidcConfig := newOIDCConfig("8090", func(s *v3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
	})
	testToken := &v3.Token{UserID: userId, AuthProvider: providerName}
	o := CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			oidc.OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*v3.OIDCConfig, error) { return oidcConfig, nil },
			},
		},
	}
	b, err := json.Marshal(&v3.OIDCConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: "R_OIDC_ID", Value: "test-id-token"})

	nr := &normanRecorder{}
	apiContext := &types.APIContext{
		Method:         req.Method,
		Request:        req,
		Query:          url.Values{},
		ResponseWriter: nr,
	}

	require.NoError(t, o.LogoutAll(apiContext, testToken))
	wantData := map[string]any{
		"idpRedirectUrl": "http://localhost:8090/user/logout?client_id=test&logout_uri=https%3A%2F%2Fexample.com%2Flogged-out",
		"type":           "oidcConfigLogoutOutput",
	}

	require.Equal(t, []normanResponse{{code: http.StatusOK, data: wantData}}, nr.responses)
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
			oidc.OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*v3.OIDCConfig, error) { return oidcConfig, nil },
			},
		},
	}
	b, err := json.Marshal(&v3.OIDCConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: "R_OIDC_ID", Value: "test-id-token"})

	nr := &normanRecorder{}
	apiContext := &types.APIContext{
		Method:         req.Method,
		Request:        req,
		Query:          url.Values{},
		ResponseWriter: nr,
	}

	require.NoError(t, o.LogoutAll(apiContext, testToken))
	wantData := map[string]any{
		"idpRedirectUrl": "",
		"type":           "oidcConfigLogoutOutput",
	}
	require.Equal(t, []normanResponse{{code: http.StatusOK, data: wantData}}, nr.responses)
}

// The Cognito logout endpoint doesn't use the ID token.
func TestLogoutAllNoIDToken(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "oidc"
	)
	oidcConfig := newOIDCConfig("8090", func(s *v3.OIDCConfig) {
		s.EndSessionEndpoint = "http://localhost:8090/user/logout"
	})

	testToken := &v3.Token{UserID: userId, AuthProvider: providerName}
	o := CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			oidc.OpenIDCProvider{
				Name:      providerName,
				GetConfig: func() (*v3.OIDCConfig, error) { return oidcConfig, nil },
			},
		},
	}

	b, err := json.Marshal(&v3.OIDCConfigLogoutInput{
		FinalRedirectURL: "https://example.com/logged-out",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(b))

	nr := &normanRecorder{}
	apiContext := &types.APIContext{
		Method:         req.Method,
		Request:        req,
		Query:          url.Values{},
		ResponseWriter: nr,
	}

	require.NoError(t, o.LogoutAll(apiContext, testToken))
	wantData := map[string]any{
		"idpRedirectUrl": "http://localhost:8090/user/logout?client_id=test&logout_uri=https%3A%2F%2Fexample.com%2Flogged-out",
		"type":           "oidcConfigLogoutOutput",
	}
	require.Equal(t, []normanResponse{{code: http.StatusOK, data: wantData}}, nr.responses)
}

func TestLogout(t *testing.T) {
	const (
		userId       string = "testing-user"
		providerName string = "keycloak"
	)

	logoutTests := map[string]struct {
		config *v3.OIDCConfig
		verify func(t require.TestingT, err error, msgAndArgs ...interface{})
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
					oidc.OpenIDCProvider{
						Name:      providerName,
						GetConfig: func() (*v3.OIDCConfig, error) { return tt.config, nil },
					},
				},
			}

			b, err := json.Marshal(&v3.OIDCConfigLogoutInput{
				FinalRedirectURL: "https://example.com/logged-out",
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(b))
			req.AddCookie(&http.Cookie{Name: "R_OIDC_ID", Value: "test-id-token"})

			nr := &normanRecorder{}
			apiContext := &types.APIContext{
				Method:         req.Method,
				Request:        req,
				Query:          url.Values{},
				ResponseWriter: nr,
			}
			tt.verify(t, o.Logout(apiContext, testToken))
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
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// normanRecorder is like httptest.ResponseRecorder, but for norman's types.ResponseWriter interface
type normanRecorder struct {
	responses []normanResponse
}

func (n *normanRecorder) Write(_ *types.APIContext, code int, obj interface{}) {
	n.responses = append(n.responses, normanResponse{
		code: code,
		data: obj,
	})
}

type normanResponse struct {
	code int
	data any
}
