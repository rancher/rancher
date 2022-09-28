package azure

import (
	"github.com/rancher/norman/httperror"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// migrateToMicrosoftGraph performs the migration of the registered Azure AD auth provider
// from the deprecated Azure AD Graph API to the Microsoft Graph API.
// It modifies the existing auth config value in the database, so that it has up-to-date endpoints to the new API.
// Most importantly, it sets the annotation that specifies that the auth config has been migrated to use the new auth flow.

// It also verifies that admins have properly configured an existing app registration's permissions
// in the Azure portal before they update their Azure AD auth config to use the new authentication flow.
// The method receives the current AuthConfig from the database, then updates it in-memory to use the new endpoints,
// and creates a new test Azure client, thereby getting an access token to the Graph API.
// Then it parses the JWT and inspects the permissions contained within. If the admins had not set those as per docs,
// then Rancher won't find the permissions in the test token and will return an error.

// If validation and applying work, then migrateToMicrosoftGraph deletes all secrets with access tokens to the
// deprecated Azure AD Graph API.
func (ap *azureProvider) migrateToMicrosoftGraph() error {
	cfg, err := ap.updateConfigAndValidatePermissions()
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

func (ap *azureProvider) updateConfigAndValidatePermissions() (*v32.AzureADConfig, error) {
	cfg, err := ap.getAzureConfigK8s()
	if err != nil {
		return nil, err
	}
	if !authProviderEnabled(cfg) {
		return nil, httperror.NewAPIError(httperror.InvalidState, "the Azure AD auth provider is not enabled")
	}

	updateAzureADEndpoints(cfg)

	// Try to get a new client, which will fetch a new access token and validate its permissions.
	_, err = clients.NewMSGraphClient(cfg, ap.secrets)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (ap *azureProvider) applyUpdatedConfig(cfg *v32.AzureADConfig) error {
	if cfg.ObjectMeta.Annotations == nil {
		cfg.ObjectMeta.Annotations = make(map[string]string)
	}
	cfg.ObjectMeta.Annotations[GraphEndpointMigratedAnnotation] = "true"
	return ap.saveAzureConfigK8s(cfg)
}

// deleteUserAccessTokens attempts to delete all secrets that contain users' access tokens used for working with
// the deprecated Azure AD Graph API.
// It is not possible to filter secrets easily by presence of specific key(s) in the data object. The method fetches all
// Opaque secrets in the relevant namespace and looks at the target key in the data to find a secret that stores a user's
// access token to delete.
func (ap *azureProvider) deleteUserAccessTokens() {
	secrets, err := ap.secrets.ListNamespaced(tokens.SecretNamespace, metav1.ListOptions{FieldSelector: "type=Opaque"})
	if err != nil {
		logrus.Errorf("failed to fetch secrets: %v", err)
		return
	}
	// Provider name for Azure AD is the main key on secret data. This allows to identify the secrets to be deleted.
	const key = Name
	for _, secret := range secrets.Items {
		if _, keyPresent := secret.Data[key]; keyPresent {
			err := ap.secrets.DeleteNamespaced(tokens.SecretNamespace, secret.Name, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("failed to delete secret %s:%s - %v", tokens.SecretNamespace, secret.Name, err)
			}
		}
	}
}
