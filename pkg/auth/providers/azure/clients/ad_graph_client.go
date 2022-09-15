package clients

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type azureADGraphClient struct {
	servicePrincipal adal.ServicePrincipalToken
	userClient       graphrbac.UsersClient
	groupClient      graphrbac.GroupsClient
}

// GetUser takes a user ID and fetches the user principal from the Azure AD Graph API.
func (c *azureADGraphClient) GetUser(id string) (v3.Principal, error) {
	user, err := c.userClient.Get(context.Background(), id)
	if err != nil {
		return v3.Principal{}, err
	}
	return c.userToPrincipal(user)
}

// ListUsers fetches all user principals in a directory from the Azure AD Graph API.
func (c *azureADGraphClient) ListUsers(filter string) ([]v3.Principal, error) {
	users, err := c.userClient.List(context.Background(), filter, "")
	if err != nil {
		return nil, err
	}

	var principals []v3.Principal
	for _, u := range users.Values() {
		principal, err := c.userToPrincipal(u)
		if err != nil {
			return nil, err
		}
		principals = append(principals, principal)
	}
	return principals, err
}

// GetGroup takes a group ID and fetches the group principal from the Azure AD Graph API.
func (c *azureADGraphClient) GetGroup(id string) (v3.Principal, error) {
	group, err := c.groupClient.Get(context.Background(), id)
	if err != nil {
		return v3.Principal{}, err
	}
	return c.groupToPrincipal(group)
}

// ListGroups fetches all group principals in a directory from the Azure AD Graph API.
func (c *azureADGraphClient) ListGroups(filter string) ([]v3.Principal, error) {
	groups, err := c.groupClient.List(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var principals []v3.Principal
	for _, u := range groups.Values() {
		principal, err := c.groupToPrincipal(u)
		if err != nil {
			return nil, err
		}
		principals = append(principals, principal)
	}
	return principals, err
}

// ListGroupMemberships takes a user ID and fetches the user's group principals as strings IDs from the Azure AD Graph API.
func (c *azureADGraphClient) ListGroupMemberships(id string) ([]string, error) {
	securityEnabledOnly := false
	params := graphrbac.UserGetMemberGroupsParameters{
		SecurityEnabledOnly: &securityEnabledOnly,
	}
	groups, err := c.userClient.GetMemberGroups(context.Background(), id, params)
	if err != nil {
		return nil, err
	}
	return *groups.Value, nil
}

// LoginUser fetches the user principal, user's group principals, and provider access token.
func (c *azureADGraphClient) LoginUser(_ *v32.AzureADConfig, _ *v32.AzureADLogin) (v3.Principal, []v3.Principal, string, error) {
	oid, err := ExtractFieldFromJWT(c.AccessToken(), "oid")
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	logrus.Debug("[AZURE_PROVIDER] Started getting user info from AzureAD")
	userPrincipal, err := c.GetUser(oid.(string))
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	userPrincipal.Me = true
	logrus.Debug("[AZURE_PROVIDER] Completed getting user info from AzureAD")

	userGroups, err := c.ListGroupMemberships(GetPrincipalID(userPrincipal))
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	groupPrincipals, err := UserGroupsToPrincipals(c, userGroups)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	providerToken, err := c.MarshalTokenJSON()
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	return userPrincipal, groupPrincipals, providerToken, nil
}

// NewADGraphClientFromCredential configures the SPT, user, and group clients using a credential.
func NewADGraphClientFromCredential(config *v32.AzureADConfig, credential *v32.AzureADLogin) (AzureClient, error) {
	var c azureADGraphClient
	logrus.Debug("[AZURE_PROVIDER] Started token swap with AzureAD")
	oauthConfig, err := adal.NewOAuthConfig(config.Endpoint, config.TenantID)
	if err != nil {
		return nil, err
	}

	// The tenantID should not be in the endpoint, drop /tenantID
	tenant := config.TenantID
	if strings.Contains(config.GraphEndpoint, tenant) {
		i := strings.Index(config.GraphEndpoint, tenant)
		config.GraphEndpoint = config.GraphEndpoint[:i-1]
	}

	spt, err := adal.NewServicePrincipalTokenFromAuthorizationCode(
		*oauthConfig,
		config.ApplicationID,
		config.ApplicationSecret,
		credential.Code,
		config.RancherURL,
		config.GraphEndpoint,
	)
	if err != nil {
		return nil, err
	}

	if err := spt.Refresh(); err != nil {
		return nil, err
	}
	logrus.Debug("[AZURE_PROVIDER] Completed token swap with AzureAD")

	c.setInternalFields(config, *spt)
	return &c, err
}

// NewAzureADGraphClientFromADALToken returns an Azure AD Graph client.
// It sets up the SPT, user and group client using an access token to Azure AD Graph API.
func NewAzureADGraphClientFromADALToken(config *v32.AzureADConfig, adalTokenSecret string) (AzureClient, error) {
	adalToken, err := unmarshalADALToken(adalTokenSecret)
	if err != nil {
		return nil, err
	}
	ac := &azureADGraphClient{}

	oauthConfig, err := adal.NewOAuthConfig(config.Endpoint, config.TenantID)
	if err != nil {
		return nil, err
	}

	secret := &adal.ServicePrincipalAuthorizationCodeSecret{
		ClientSecret: config.ApplicationSecret,
	}

	spt, err := adal.NewServicePrincipalTokenFromManualTokenSecret(
		*oauthConfig,
		config.ApplicationID,
		config.GraphEndpoint,
		adalToken,
		secret)

	if err != nil {
		return nil, err
	}

	ac.setInternalFields(config, *spt)
	return ac, nil
}

func (c *azureADGraphClient) setInternalFields(config *v32.AzureADConfig, spt adal.ServicePrincipalToken) {
	c.servicePrincipal = spt

	// Create the required bearer token.
	bearer := autorest.NewBearerAuthorizer(&spt)

	// Set up the user client.
	userClient := graphrbac.NewUsersClientWithBaseURI(config.GraphEndpoint, config.TenantID)
	userClient.Authorizer = bearer
	c.userClient = userClient

	// Set up the group client.
	groupClient := graphrbac.NewGroupsClientWithBaseURI(config.GraphEndpoint, config.TenantID)
	groupClient.Authorizer = bearer
	c.groupClient = groupClient
}

// AccessToken returns the OAuthToken from the underlying SPT.
func (c *azureADGraphClient) AccessToken() string {
	return c.servicePrincipal.OAuthToken()
}

// MarshalTokenJSON returns the JSON representation of the underlying access token.
func (c *azureADGraphClient) MarshalTokenJSON() (string, error) {
	b, err := c.servicePrincipal.MarshalTokenJSON()
	return string(b), err
}

func (c *azureADGraphClient) userToPrincipal(user graphrbac.User) (v3.Principal, error) {
	return v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: Name + "_user://" + *user.ObjectID},
		DisplayName:   *user.DisplayName,
		LoginName:     *user.UserPrincipalName,
		PrincipalType: "user",
		Provider:      Name,
	}, nil
}

func (c *azureADGraphClient) groupToPrincipal(group graphrbac.ADGroup) (v3.Principal, error) {
	return v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: Name + "_group://" + *group.ObjectID},
		DisplayName:   *group.DisplayName,
		PrincipalType: "group",
		Provider:      Name,
	}, nil
}
