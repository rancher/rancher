package data

import (
	"encoding/json"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/keycloakoidc"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	localprovider "github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func AuthConfigs(management *config.ManagementContext) error {
	if err := addAuthConfig(github.Name, client.GithubConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(activedirectory.Name, client.ActiveDirectoryConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(azure.Name, client.AzureADConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(ldap.OpenLdapName, client.OpenLdapConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(ldap.FreeIpaName, client.FreeIpaConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfigWithSLO(saml.PingName, client.PingConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfigWithSLO(saml.ADFSName, client.ADFSConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfigWithSLO(saml.KeyCloakName, client.KeyCloakConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfigWithSLO(saml.OKTAName, client.OKTAConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfigWithSLO(saml.ShibbolethName, client.ShibbolethConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(googleoauth.Name, client.GoogleOauthConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(oidc.Name, client.OIDCConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(keycloakoidc.Name, client.KeyCloakOIDCConfigType, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(genericoidc.Name, client.GenericOIDCConfigType, false, management); err != nil {
		return err
	}

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
	if name == azure.Name {
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
