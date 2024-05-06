package data

import (
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
)

func AuthConfigs(management *config.ManagementContext) error {
	if err := addAuthConfig(github.Name, client.GithubConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(activedirectory.Name, client.ActiveDirectoryConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(azure.Name, client.AzureADConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(ldap.OpenLdapName, client.OpenLdapConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(ldap.FreeIpaName, client.FreeIpaConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(saml.PingName, client.PingConfigType, false, true, management); err != nil {
		return err
	}

	if err := addAuthConfig(saml.ADFSName, client.ADFSConfigType, false, true, management); err != nil {
		return err
	}

	if err := addAuthConfig(saml.KeyCloakName, client.KeyCloakConfigType, false, true, management); err != nil {
		return err
	}

	if err := addAuthConfig(saml.OKTAName, client.OKTAConfigType, false, true, management); err != nil {
		return err
	}

	if err := addAuthConfig(saml.ShibbolethName, client.ShibbolethConfigType, false, true, management); err != nil {
		return err
	}

	if err := addAuthConfig(googleoauth.Name, client.GoogleOauthConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(oidc.Name, client.OIDCConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(keycloakoidc.Name, client.KeyCloakOIDCConfigType, false, false, management); err != nil {
		return err
	}

	if err := addAuthConfig(genericoidc.Name, client.GenericOIDCConfigType, false, false, management); err != nil {
		return err
	}

	return addAuthConfig(localprovider.Name, client.LocalConfigType, true, false, management)
}

func addAuthConfig(name, aType string, enabled, sloSupported bool, management *config.ManagementContext) error {
	annotations := make(map[string]string)
	if name == azure.Name {
		annotations[azure.GraphEndpointMigratedAnnotation] = "true"
	}
	annotations[auth.CleanupAnnotation] = auth.CleanupRancherLocked

	_, err := management.Management.AuthConfigs("").ObjectClient().Create(&v3.AuthConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Type:               aType,
		Enabled:            enabled,
		LogoutAllSupported: sloSupported,
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
