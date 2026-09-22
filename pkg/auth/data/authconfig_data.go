package data

import (
	"encoding/json"

	"github.com/rancher/rancher/pkg/auth/providers/azure"
	localprovider "github.com/rancher/rancher/pkg/auth/providers/local"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func AuthConfigs(management *config.ManagementContext) error {
	// KEVIN: TODO: Remove these.
	// KEVIN: We need to figure out how to provide a list of possible
	// AuthConfigs
	// if err := addAuthConfig(github.ProviderName, client.GithubConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfig(githubapp.ProviderName, client.GithubAppConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfig(activedirectory.ProviderName, client.ActiveDirectoryConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(azure.ProviderName, client.AzureADConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfig(ldap.OpenLdapName, client.OpenLdapConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfig(ldap.FreeIpaName, client.FreeIpaConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(saml.PingName, client.PingConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(saml.ADFSName, client.ADFSConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(saml.KeyCloakName, client.KeyCloakConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(saml.OKTAName, client.OKTAConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(saml.ShibbolethName, client.ShibbolethConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(saml.GenericSAMLName, client.GenericSAMLConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfig(googleoauth.ProviderName, client.GoogleOauthConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(oidc.Name, client.OIDCConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(keycloakoidc.ProviderName, client.KeyCloakOIDCConfigType, false, management); err != nil {
	// 	return err
	// }

	// if err := addAuthConfigWithSLO(genericoidc.ProviderName, client.GenericOIDCConfigType, false, management); err != nil {
	// 	return err
	// }
	// if err := addAuthConfigWithSLO(cognito.Name, client.CognitoConfigType, false, management); err != nil {
	// 	return err
	// }

	return addAuthConfig(localprovider.Name, client.LocalConfigType, true, management)
}

func addAuthConfig(name, aType string, enabled bool, management *config.ManagementContext) error {
	return addAuthConfigCore(name, aType, enabled, false, management)
}

func addAuthConfigWithSLO(name, aType string, enabled bool, management *config.ManagementContext) error {
	return addAuthConfigCore(name, aType, enabled, true, management)
}

func addAuthConfigCore(name, aType string, enabled, sloSupported bool, management *config.ManagementContext) error {
	annotations := make(map[string]string)
	if name == azure.ProviderName {
		annotations[azure.GraphEndpointMigratedAnnotation] = "true"
	}
	annotations[auth.CleanupAnnotation] = auth.CleanupRancherLocked

	createdOrKnown, err := management.Management.AuthConfigs("").ObjectClient().Create(&v3.AuthConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Type:               aType,
		Enabled:            enabled,
		LogoutAllSupported: sloSupported,
	})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		// Make sure the logoutAllSupported field is set correctly for the existing authConfig.
		// Use patch to avoid fetching the object first.
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{{
			Op:    "add",
			Path:  "/logoutAllSupported",
			Value: sloSupported,
		}})
		if err != nil {
			return err
		}

		_, err = management.Management.AuthConfigs("").ObjectClient().
			Patch(name, createdOrKnown, types.JSONPatchType, patch)
		if err != nil {
			return err
		}
	}

	return nil
}
