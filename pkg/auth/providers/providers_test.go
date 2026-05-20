package providers

import (
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/stretchr/testify/assert"
)

func TestIsSAMLProvider(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"ping", "pingConfig", "pingProvider",
		"adfs", "adfsConfig", "adfsProvider",
		"keycloak", "keycloakConfig", "keycloakProvider",
		"okta", "oktaConfig", "oktaProvider",
		"shibboleth", "shibbolethConfig", "shibbolethProvider",
	} {
		assert.True(t, IsSAMLProviderType(name), name)
	}

	for _, name := range []string{
		"github", "githubConfig", "githubProvider",
		"local", "localConfig", "localProvider",
	} {
		assert.False(t, IsSAMLProviderType(name), name)
	}
}

func TestProviderUsesUserSecrets(t *testing.T) {
	SetProviders(map[string]common.AuthProvider{
		github.Name:    &github.Provider{},
		githubapp.Name: &githubapp.Provider{},
		local.Name:     &local.Provider{},
	})
	defer SetProviders(nil)

	assert.True(t, ProviderUsesUserSecrets(github.Name))
	assert.False(t, ProviderUsesUserSecrets(githubapp.Name))
	assert.False(t, ProviderUsesUserSecrets(local.Name))
}

func TestProviderCanRefreshPrincipals(t *testing.T) {
	SetProviders(map[string]common.AuthProvider{
		github.Name:      &github.Provider{},
		genericoidc.Name: &genericoidc.GenOIDCProvider{},
	})
	defer SetProviders(nil)

	assert.True(t, ProviderCanRefreshPrincipals(github.Name))
	assert.False(t, ProviderCanRefreshPrincipals(genericoidc.Name))
}
