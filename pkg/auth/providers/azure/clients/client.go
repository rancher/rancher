package clients

import (
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

// Name is the identifier for the Azure AD auth provider.
const Name = "azuread"

// AzureClient specifies the subset of operations that a real client would delegate to some SDK for accessing
// one of the two APIs to work with Active Directory resources - Azure AD Graph and Microsoft Graph.
type AzureClient interface {
	LoginUser(*v32.AzureADConfig, *v32.AzureADLogin) (v3.Principal, []v3.Principal, string, error)
	AccessToken() string
	MarshalTokenJSON() (string, error)

	GetUser(id string) (v3.Principal, error)
	ListUsers(filter string) ([]v3.Principal, error)
	GetGroup(id string) (v3.Principal, error)
	ListGroups(filter string) ([]v3.Principal, error)
	ListGroupMemberships(id string) ([]string, error)
}

// NewAzureClientFromCredential returns a new client to be used for first-time authentication. This means it does not need any
// externally stored secrets with tokens, as is the case when a client is created to use an existing access token.
func NewAzureClientFromCredential(config *v32.AzureADConfig, useDeprecatedAzureADClient bool, credential *v32.AzureADLogin, secrets corev1.SecretInterface) (AzureClient, error) {
	if useDeprecatedAzureADClient {
		return NewADGraphClientFromCredential(config, credential)
	}
	return NewMSGraphClient(config, secrets)
}

// NewAzureClientFromSecret returns a new client to be used for search and other activities after initial authentication.
// The client would fetch the access token from either a refresh token or secret contents passed in.
func NewAzureClientFromSecret(config *v32.AzureADConfig, useDeprecatedAzureADClient bool, secret string, secrets corev1.SecretInterface) (AzureClient, error) {
	if useDeprecatedAzureADClient {
		return NewAzureADGraphClientFromADALToken(config, secret)
	}
	return NewMSGraphClient(config, secrets)
}
