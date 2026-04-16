package keycloakoidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/core"
)

func TestKeycloakOIDCProvider_SearchPrincipals(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	expectedResult := []apiv3.Principal{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "keycloakoidc_user://9f3f3bab-1c7f-4f1e-970c-6bd2db77684b",
			},
			DisplayName:   "Testing",
			LoginName:     "testing",
			PrincipalType: UserType,
			Provider:      Name,
		},
	}

	t.Run("test search for user principal with client authenticated search", func(t *testing.T) {
		testSrv := newFakeKeycloakServer(t, privateKey, func(t *testing.T, r *http.Request) bool {
			bearerString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(bearerString, &claims, nil)
			// the client authentication doesn't use an Access Token so it has
			// no claims.
			return errors.Is(err, jwt.ErrTokenUnverifiable) && claims["auth_provider"] == nil

		})
		oidcConfig := testOIDCConfig(testSrv.URL, func(o *v3.OIDCConfig) {
			o.ClientAuthenticatedSearch = true
		})
		var createdSecret string
		createTokenManager := &fakeTokenManager{
			getSecretFunc: func(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error) {
				return "", apierrors.NewNotFound(core.Resource("Secret"), "cattle-tokens/"+provider)
			},
			createSecretFunc: func(userID, provider, secret string) error {
				createdSecret = secret
				return nil
			},
		}
		g := &keyCloakOIDCProvider{
			oidc.OpenIDCProvider{
				Name:     Name,
				Type:     client.KeyCloakOIDCConfigType,
				TokenMgr: createTokenManager,
			},
		}
		g.GetConfig = func() (*apiv3.OIDCConfig, error) {
			return oidcConfig, nil
		}

		result, err := g.SearchPrincipals("user1", UserType, &apiv3.Token{})
		require.NoError(t, err, "SearchPrincipals() returned an error")
		assert.Equal(t, expectedResult, result)
		assert.NotEmpty(t, createdSecret)
	})

	t.Run("test search for user principal with client authenticated search when token secret exists", func(t *testing.T) {
		testSrv := newFakeKeycloakServer(t, privateKey, func(t *testing.T, r *http.Request) bool {
			bearerString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(bearerString, &claims, nil)
			// the client authentication doesn't use an Access Token so it has
			// no claims.
			return errors.Is(err, jwt.ErrTokenUnverifiable) && claims["auth_provider"] == nil

		})
		oidcConfig := testOIDCConfig(testSrv.URL, func(o *v3.OIDCConfig) {
			o.ClientAuthenticatedSearch = true
		})
		createTokenManager := &fakeTokenManager{
			getSecretFunc: func(userID, provider string, fallbackTokens []accessor.TokenAccessor) (string, error) {
				return `{"access_token":"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwOi8vMTI3LjAuMC4xOjQyMDc5IiwiYXVkIjpbInRlc3QiXSwiZXhwIjoxNzczNDA1NDA5fQ.Omk646OoB9XGwC4RQIhGwrfpwj3mE73SB29l2A24KOaJ3s8HoC8JjfD6xRsDscE2MSW0Y6MI7lZodkToChBYk5drT8wU0_kOHgOey4vfobDvXpS7NhH0ECXmQ_HjxCpA651gFQYa1FyUr2exOmult5MTZOeTbFaxZYz6MSmjM4YcNrXkknZAwsS6FdX5kMVhcrGnAzi387QF8Kt0UnGlsmb-6oZ82Fw8-XXpnhNBB7KCgC1ehWwnEt2z1CXjIvAqKTBiGsd7vzbuDn-H9eqpii2_lEACCIW3PM0SNd49tMnF_JjdPjb_LdUKUW_0n7dDVm5-VRAfrlU9fgQJBKSipw","refresh_token":"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwOi8vMTI3LjAuMC4xOjQyMDc5IiwiYXVkIjpbInRlc3QiXSwiZXhwIjoxNzczNDA1NDA5fQ.Omk646OoB9XGwC4RQIhGwrfpwj3mE73SB29l2A24KOaJ3s8HoC8JjfD6xRsDscE2MSW0Y6MI7lZodkToChBYk5drT8wU0_kOHgOey4vfobDvXpS7NhH0ECXmQ_HjxCpA651gFQYa1FyUr2exOmult5MTZOeTbFaxZYz6MSmjM4YcNrXkknZAwsS6FdX5kMVhcrGnAzi387QF8Kt0UnGlsmb-6oZ82Fw8-XXpnhNBB7KCgC1ehWwnEt2z1CXjIvAqKTBiGsd7vzbuDn-H9eqpii2_lEACCIW3PM0SNd49tMnF_JjdPjb_LdUKUW_0n7dDVm5-VRAfrlU9fgQJBKSipw","expiry":"0001-01-01T00:00:00Z"}`, nil
			},
		}
		g := &keyCloakOIDCProvider{
			oidc.OpenIDCProvider{
				Name:     Name,
				Type:     client.KeyCloakOIDCConfigType,
				TokenMgr: createTokenManager,
			},
		}
		g.GetConfig = func() (*apiv3.OIDCConfig, error) {
			return oidcConfig, nil
		}

		result, err := g.SearchPrincipals("user1", UserType, &apiv3.Token{})
		require.NoError(t, err, "SearchPrincipals() returned an error")
		assert.Equal(t, expectedResult, result)
	})

	t.Run("test search for user principal user authenticated search", func(t *testing.T) {
		testSrv := newFakeKeycloakServer(t, privateKey, func(t *testing.T, r *http.Request) bool {
			bearerString := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(bearerString, &claims, nil)
			return errors.Is(err, jwt.ErrTokenUnverifiable) && claims["auth_provider"] == "auth-provider"
		})
		oidcConfig := testOIDCConfig(testSrv.URL, func(o *v3.OIDCConfig) {
			o.ClientAuthenticatedSearch = false
		})
		fakeAccessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"aud":           []interface{}{"client-id"},
			"exp":           float64(time.Now().Add(10 * time.Hour).Unix()),
			"iss":           oidcConfig.Issuer,
			"iat":           float64(time.Now().Unix()),
			"sub":           "test-user",
			"auth_provider": "auth-provider",
			"scope":         []string{"openid", "profile"},
		})
		fakeAccessTokenString, err := fakeAccessToken.SignedString(privateKey)
		require.NoError(t, err, "Failed to sign fake access token")
		createTokenManager := &fakeTokenManager{
			getSecretFunc: func(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error) {
				b, err := json.Marshal(map[string]string{
					"access_token": fakeAccessTokenString,
				})

				return string(b), err
			},
		}
		g := &keyCloakOIDCProvider{
			oidc.OpenIDCProvider{
				Name:     Name,
				Type:     client.KeyCloakOIDCConfigType,
				TokenMgr: createTokenManager,
			},
		}
		g.GetConfig = func() (*apiv3.OIDCConfig, error) {
			return oidcConfig, nil
		}
		result, err := g.SearchPrincipals("user1", UserType, &apiv3.Token{
			ProviderInfo: map[string]string{
				"access_token": fakeAccessTokenString,
			},
		})
		require.NoError(t, err, "SearchPrincipals() returned an error")
		assert.Equal(t, result, expectedResult)
	})
}

func TestKeyCloakOIDCProvider_TransformToAuthProvider(t *testing.T) {
	tests := map[string]struct {
		authConfig map[string]any
		expected   map[string]any
	}{
		"Test with valid authConfig": {
			authConfig: map[string]any{
				"metadata":     map[string]any{"name": "keycloakoidc"},
				"clientId":     "client123",
				"rancherUrl":   "https://example.com/callback",
				"scope":        "openid profile email",
				"issuer":       "https://ranchertest.io/issuer",
				"authEndpoint": "https://ranchertest.io/auth",
			},
			expected: map[string]any{
				"id":                 "keycloakoidc",
				"redirectUrl":        "https://ranchertest.io/auth?client_id=client123&response_type=code&redirect_uri=https://example.com/callback",
				"scopes":             "openid profile email",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		"When configuration has acrValue": {
			authConfig: map[string]any{
				"metadata":     map[string]any{"name": "keycloakoidc"},
				"clientId":     "client123",
				"rancherUrl":   "https://example.com/callback",
				"scope":        "openid profile email",
				"issuer":       "https://ranchertest.io/issuer",
				"authEndpoint": "https://ranchertest.io/auth",
				"acrValue":     "testing",
			},
			expected: map[string]any{
				"id":                 "keycloakoidc",
				"redirectUrl":        "https://ranchertest.io/auth?client_id=client123&response_type=code&redirect_uri=https://example.com/callback",
				"scopes":             "openid profile email",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
	}

	provider := &keyCloakOIDCProvider{
		oidc.OpenIDCProvider{},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := provider.TransformToAuthProvider(test.authConfig)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		})
	}
}

func testOIDCConfig(baseURL string, opts ...func(*v3.OIDCConfig)) *v3.OIDCConfig {
	issuerURL := baseURL + "/realms/testing"
	cfg := &v3.OIDCConfig{
		Issuer:           issuerURL,
		ClientID:         "test",
		JWKSUrl:          issuerURL + "/.well-known/jwks.json",
		AuthEndpoint:     issuerURL + "/auth",
		TokenEndpoint:    issuerURL + "/token",
		UserInfoEndpoint: issuerURL + "/user",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

type fakeTokenManager struct {
	getSecretFunc               func(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error)
	createTokenAndSetCookieFunc func(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error
	createSecretFunc            func(userID, provider, secret string) error
	updateSecretFunc            func(userID, provider, secret string) error
}

func (m *fakeTokenManager) GetSecret(userID, provider string, fallbackTokens []accessor.TokenAccessor) (string, error) {
	if m.getSecretFunc == nil {
		return "", errors.New("GetSecret called but no function provided")
	}

	return m.getSecretFunc(userID, provider, fallbackTokens)
}

func (m *fakeTokenManager) CreateSecret(userID, provider, secret string) error {
	if m.createSecretFunc == nil {
		return errors.New("CreateSecret called but no function provided")
	}

	return m.createSecretFunc(userID, provider, secret)
}

func (m *fakeTokenManager) CreateTokenAndSetCookie(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error {
	if m.createTokenAndSetCookieFunc == nil {
		return errors.New("CreateTokenAndSetCookie called but no function provided")
	}

	return m.createTokenAndSetCookieFunc(userID, userPrincipal, groupPrincipals, providerToken, ttl, description, request)
}

func (m *fakeTokenManager) UpdateSecret(userID, provider, secret string) error {
	if m.updateSecretFunc == nil {
		return errors.New("UpdateSecret called but no function provided")
	}

	return m.updateSecretFunc(userID, provider, secret)
}
