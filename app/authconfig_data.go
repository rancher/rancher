package app

import (
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	localprovider "github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addAuthConfigs(management *config.ManagementContext) error {
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

	if err := addAuthConfig(saml.PingName, client.PingConfigType, false, management); err != nil {
		return err
	}

	return addAuthConfig(localprovider.Name, client.LocalConfigType, true, management)
}

func addAuthConfig(name, aType string, enabled bool, management *config.ManagementContext) error {
	_, err := management.Management.AuthConfigs("").ObjectClient().Create(&v3.AuthConfig{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Type:    aType,
		Enabled: enabled,
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
