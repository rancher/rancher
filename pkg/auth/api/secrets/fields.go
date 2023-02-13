package secrets

import (
	azuread "github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

var (
	// TypeToFields associates an Auth Config type with a set of secret names related to the config.
	TypeToFields = map[string][]string{
		client.GithubConfigType:          {client.GithubConfigFieldClientSecret},
		client.ActiveDirectoryConfigType: {client.ActiveDirectoryConfigFieldServiceAccountPassword},
		client.AzureADConfigType:         {client.AzureADConfigFieldApplicationSecret},
		client.OpenLdapConfigType:        {client.LdapConfigFieldServiceAccountPassword},
		client.FreeIpaConfigType:         {client.LdapConfigFieldServiceAccountPassword},
		client.PingConfigType:            {client.PingConfigFieldSpKey},
		client.ADFSConfigType:            {client.ADFSConfigFieldSpKey},
		client.KeyCloakConfigType:        {client.KeyCloakConfigFieldSpKey},
		client.OKTAConfigType:            {client.OKTAConfigFieldSpKey},
		client.ShibbolethConfigType:      {client.ShibbolethConfigFieldSpKey},
		client.GoogleOauthConfigType:     {client.GoogleOauthConfigFieldOauthCredential, client.GoogleOauthConfigFieldServiceAccountCredential},
		client.OIDCConfigType:            {client.OIDCConfigFieldPrivateKey, client.OIDCConfigFieldClientSecret},
		client.KeyCloakOIDCConfigType:    {client.KeyCloakOIDCConfigFieldPrivateKey, client.KeyCloakOIDCConfigFieldClientSecret},
	}
	// SubTypeToFields associates an Auth Config type with a nested map of secret names related to the config.
	SubTypeToFields = map[string]map[string][]string{
		client.ShibbolethConfigType: {
			client.ShibbolethConfigFieldOpenLdapConfig: {client.LdapConfigFieldServiceAccountPassword},
		},
	}

	// NameToFields keeps track of secrets that Rancher must clean up for the given auth provider specified by name.
	NameToFields = map[string][]string{
		azuread.Name: {"access-token"},
	}
)
