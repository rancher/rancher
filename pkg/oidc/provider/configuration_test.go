package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func TestOpenIDConfigurationEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	err := settings.ServerURL.Set("https://rancher.com")
	assert.NoError(t, err)

	openIDConfigurationEndpoint(rec, &http.Request{})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"issuer":"https://rancher.com/oidc","authorization_endpoint":"https://rancher.com/oidc/authorize","token_endpoint":"https://rancher.com/oidc/token","userinfo_endpoint":"https://rancher.com/oidc/userinfo","jwks_uri":"https://rancher.com/oidc/.well-known/jwks.json","response_types_supported":["code"],"subject_types_supported":["public"],"id_token_signing_alg_values_supported":["RS256"],"code_challenge_methods_supported":["S256"],"scopes_supported":["openid","profile","offline_access"],"grant_types_supported":["authorization_code","refresh_token"]}`, strings.TrimSpace(rec.Body.String()))
}
