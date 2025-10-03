package clients

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

// Name is the identifier for the Azure AD auth provider.
const Name = "azuread"

// AzureClient specifies the subset of operations that a real client would delegate to some SDK for accessing
// one of the two APIs to work with Active Directory resources - Azure AD Graph and Microsoft Graph.
type AzureClient interface {
	LoginUser(*v3.AzureADConfig, *v3.AzureADLogin) (v3.Principal, []v3.Principal, string, error)
	AccessToken() string
	MarshalTokenJSON() (string, error)

	GetUser(id string) (v3.Principal, error)
	ListUsers(filter string) ([]v3.Principal, error)
	GetGroup(id string) (v3.Principal, error)
	ListGroups(filter string) ([]v3.Principal, error)
	ListGroupMemberships(id string, filter string) ([]string, error)
}

// NewAzureClient returns a new client to be used for search and other activities after initial authentication.
// The client would fetch the access token from either a refresh token or secret contents passed in.
func NewAzureClient(config *v3.AzureADConfig, secrets wcorev1.SecretController) (AzureClient, error) {
	return NewMSGraphClient(config, secrets)
}
