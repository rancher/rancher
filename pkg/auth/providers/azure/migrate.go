package azure

import (
	"github.com/rancher/norman/httperror"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	"github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/sirupsen/logrus"
)

// migrateToMicrosoftGraph performs a migration of the registered Azure AD auth provider
// from the deprecated Azure AD Graph API to the Microsoft Graph API.
// It modifies the existing auth config in the database, so that it has up-to-date endpoints to the new API.
// Most importantly, it sets the annotation that specifies that the auth config has been migrated to use the new auth flow.
// The method receives the current AuthConfig from the database, then updates it in-memory to use the new endpoints.

// It also creates a new test Azure client to catch any errors before committing the migration.
// If validation and applying work, then migrateToMicrosoftGraph deletes all secrets with access tokens to the
// deprecated Azure AD Graph API.
func (ap *Provider) migrateToMicrosoftGraph() error {
	cfg, err := ap.updateConfigAndTest()
	if err != nil {
		return err
	}
	if err = ap.applyUpdatedConfig(cfg); err != nil {
		return err
	}
	ap.deleteUserAccessTokens()
	clients.GroupCache.Purge()
	return nil
}

func (ap *Provider) updateConfigAndTest() (*v32.AzureADConfig, error) {
	cfg, err := ap.GetAzureConfigK8s()
	if err != nil {
		return nil, err
	}
	if !authProviderEnabled(cfg) {
		return nil, httperror.NewAPIError(httperror.InvalidState, "the Azure AD auth provider is not enabled")
	}

	updateAzureADEndpoints(cfg)

	// Try to get a new client, which will fetch a new access token and catch any errors.
	_, err = clients.NewMSGraphClient(cfg, ap.secrets)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (ap *Provider) applyUpdatedConfig(cfg *v32.AzureADConfig) error {
	if cfg.ObjectMeta.Annotations == nil {
		cfg.ObjectMeta.Annotations = make(map[string]string)
	}
	cfg.ObjectMeta.Annotations[GraphEndpointMigratedAnnotation] = "true"
	return ap.saveAzureConfigK8s(cfg)
}

func (ap *Provider) deleteUserAccessTokens() {
	if err := secrets.CleanupOAuthTokens(ap.secrets, ap.GetName()); err != nil {
		logrus.Errorf("error during OAuth secrets clean up on Azure AD endpoint update: %v", err)
	}
}
