package common

import (
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

// logoutAllSupportedTypes lists the AuthConfig types of the providers which
// implement SSO logout (Single Log Out). Add new SLO capable provider types here.
var logoutAllSupportedTypes = map[string]bool{
	client.AzureADConfigType:      true,
	client.PingConfigType:         true,
	client.ADFSConfigType:         true,
	client.KeyCloakConfigType:     true,
	client.OKTAConfigType:         true,
	client.ShibbolethConfigType:   true,
	client.GenericSAMLConfigType:  true,
	client.OIDCConfigType:         true,
	client.KeyCloakOIDCConfigType: true,
	client.GenericOIDCConfigType:  true,
	client.CognitoConfigType:      true,
}

// SupportsLogoutAll reports whether AuthConfigs of the given type support SSO
// logout. This is a capability of the provider type, not a user setting, so it
// is the value AuthConfig.LogoutAllSupported must have.
func SupportsLogoutAll(authConfigType string) bool {
	return logoutAllSupportedTypes[authConfigType]
}
