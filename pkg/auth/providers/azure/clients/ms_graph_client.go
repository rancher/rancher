package clients

import (
	"errors"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
)

const (
	// AccessTokenSecretName is the name of the secret that contains an access token for the Microsoft Graph API.
	AccessTokenSecretName = "azuread-access-token"
)

// NewMSGraphClient creates and returns a stub client that returns errors.
// MODIFIED: Disabled Microsoft Graph SDK to avoid dependencies
func NewMSGraphClient(config *v32.AzureADConfig, secrets wcorev1.SecretController) (*AzureMSGraphClient, error) {
	return nil, errors.New("Microsoft Graph client disabled - use deprecated Azure AD Graph client instead")
}

// AzureMSGraphClient stub implementation
type AzureMSGraphClient struct{}

func (c AzureMSGraphClient) GetUser(userID string) (v3.Principal, error) {
	return v3.Principal{}, errors.New("Microsoft Graph client disabled")
}

func (c AzureMSGraphClient) ListUsers(filter string) ([]v3.Principal, error) {
	return nil, errors.New("Microsoft Graph client disabled")
}

func (c AzureMSGraphClient) GetGroup(groupID string) (v3.Principal, error) {
	return v3.Principal{}, errors.New("Microsoft Graph client disabled")
}

func (c AzureMSGraphClient) ListGroups(filter string) ([]v3.Principal, error) {
	return nil, errors.New("Microsoft Graph client disabled")
}

func (c AzureMSGraphClient) ListGroupMemberships(userID string, filter string) ([]string, error) {
	return nil, errors.New("Microsoft Graph client disabled")
}

func (c AzureMSGraphClient) LoginUser(config *v32.AzureADConfig, credential *v32.AzureADLogin) (v3.Principal, []v3.Principal, string, error) {
	return v3.Principal{}, nil, "", errors.New("Microsoft Graph client disabled")
}

func (c AzureMSGraphClient) AccessToken() string {
	return ""
}

func (c AzureMSGraphClient) MarshalTokenJSON() (string, error) {
	return "", errors.New("Microsoft Graph client disabled")
}