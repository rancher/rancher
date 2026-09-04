package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/labels"
)

func TestOpenIDConfigurationEndpoint(t *testing.T) {
	const redirectURI = "https://client.example/callback"
	ctrl := gomock.NewController(t)
	oidcClientCache := fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl)
	oidcClientCache.EXPECT().List(labels.Everything()).Return([]*v3.OIDCClient{
		{
			Spec: v3.OIDCClientSpec{
				RedirectURIs: []string{redirectURI},
			},
		},
	}, nil)
	provider := Provider{
		authHandler: &authorizeHandler{
			oidcClientCache: oidcClientCache,
		},
	}
	mux := http.NewServeMux()
	provider.RegisterOIDCProviderHandles(mux)
	rec := httptest.NewRecorder()
	err := settings.ServerURL.Set("https://rancher.com")
	assert.NoError(t, err)

	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://client.example/oidc/.well-known/openid-configuration", nil))

	assertOpenIDConfigurationResponse(t, rec)
	assert.Equal(t, http.Header{
		"Access-Control-Allow-Methods": []string{"GET, POST"},
		"Access-Control-Allow-Origin":  []string{redirectURI},
		"Content-Type":                 []string{"application/json"},
		"Referrer-Policy":              []string{"strict-origin-when-cross-origin"},
		"Strict-Transport-Security":    []string{"max-age=31536000"},
		"X-Content-Type-Options":       []string{"nosniff"},
		"X-Frame-Options":              []string{"SAMEORIGIN"},
	}, rec.Header())
}

func TestOpenIDConfigurationEndpointWithoutOIDCClients(t *testing.T) {
	ctrl := gomock.NewController(t)
	oidcClientCache := fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl)
	oidcClientCache.EXPECT().List(labels.Everything()).Return(nil, nil)
	err := settings.ServerURL.Set("https://rancher.com")
	assert.NoError(t, err)

	provider := Provider{
		authHandler: &authorizeHandler{
			oidcClientCache: oidcClientCache,
		},
	}
	mux := http.NewServeMux()
	provider.RegisterOIDCProviderHandles(mux)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oidc/.well-known/openid-configuration", nil))

	assertOpenIDConfigurationResponse(t, rec)
	assert.Equal(t, http.Header{
		// Does not contain Access-Control-Allow-Origin header because there are no OIDC clients configured.
		"Access-Control-Allow-Methods": []string{"GET, POST"},
		"Content-Type":                 []string{"application/json"},
		"Referrer-Policy":              []string{"strict-origin-when-cross-origin"},
		"Strict-Transport-Security":    []string{"max-age=31536000"},
		"X-Content-Type-Options":       []string{"nosniff"},
		"X-Frame-Options":              []string{"SAMEORIGIN"},
	}, rec.Header())
}

func assertOpenIDConfigurationResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response OpenIDConfiguration
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "https://rancher.com/oidc", response.Issuer)
	assert.Equal(t, "https://rancher.com/oidc/authorize", response.AuthorizationEndpoint)
	assert.Equal(t, "https://rancher.com/oidc/token", response.TokenEndpoint)
	assert.Equal(t, "https://rancher.com/oidc/userinfo", response.UserInfoEndpoint)
	assert.Equal(t, "https://rancher.com/oidc/.well-known/jwks.json", response.JWKSURI)
	assert.Equal(t, []string{"code"}, response.ResponseTypesSupported)
	assert.Equal(t, []string{"public"}, response.SubjectTypesSupported)
	assert.Equal(t, []string{"RS256"}, response.IDTokenSigningAlgsValuesSupported)
	assert.Equal(t, []string{"S256"}, response.CodeChallengeMethodsSupported)
	assert.Equal(t, []string{"openid", "profile", "offline_access"}, response.ScopesSupported)
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, response.GrantTypesSupported)
}
