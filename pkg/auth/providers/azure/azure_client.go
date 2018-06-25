package azure

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type azureClient struct {
	servicePrincipal *adal.ServicePrincipalToken
	userClient       graphrbac.UsersClient
	groupClient      graphrbac.GroupsClient
}

// newClientCode sets up the SPT, user and group client using a code
func newClientCode(code string, config *v3.AzureADConfig) (*azureClient, error) {
	ac := &azureClient{}

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
		code,
		config.RancherURL,
		config.GraphEndpoint,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// The refresh is required, call above just creates the struct
	spt.SetRefreshCallbacks(nil)
	err = spt.Refresh()
	if err != nil {
		return nil, err
	}

	ac.servicePrincipal = spt

	// Create the required bearer token
	bearer := autorest.NewBearerAuthorizer(spt)

	// Setup the user client
	userClient := graphrbac.NewUsersClientWithBaseURI(config.GraphEndpoint, config.TenantID)
	userClient.Authorizer = bearer

	ac.userClient = userClient

	// Setup the group client
	groupClient := graphrbac.NewGroupsClientWithBaseURI(config.GraphEndpoint, config.TenantID)
	groupClient.Authorizer = bearer

	ac.groupClient = groupClient
	return ac, nil
}

// newClientToken sets up the SPT, user and group client using a current Token
func newClientToken(token v3.Token, config *v3.AzureADConfig, azureToken adal.Token) (*azureClient, error) {
	ac := &azureClient{}

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
		azureToken,
		secret,
		nil)

	if err != nil {
		return nil, err
	}

	spt.SetRefreshCallbacks(nil)

	ac.servicePrincipal = spt

	// Create the required bearer token
	bearer := autorest.NewBearerAuthorizer(spt)

	// Setup the user client
	userClient := graphrbac.NewUsersClientWithBaseURI(config.GraphEndpoint, config.TenantID)
	userClient.Authorizer = bearer

	ac.userClient = userClient

	// Setup the group client
	groupClient := graphrbac.NewGroupsClientWithBaseURI(config.GraphEndpoint, config.TenantID)
	groupClient.Authorizer = bearer

	ac.groupClient = groupClient

	return ac, nil
}

// accessToken returns the OAuthToken from the underlying SPT
func (ac *azureClient) accessToken() string {
	return ac.servicePrincipal.OAuthToken()
}

// marshalTokenJSON returns a JSON of the underlying Token
func (ac *azureClient) marshalTokenJSON() ([]byte, error) {
	return ac.servicePrincipal.MarshalTokenJSON()
}

// parseJWTforField will parse the claims in a token for the field requested
func parseJWTforField(tokenString string, fieldID string) (string, error) {
	pieces := strings.Split(tokenString, ".")
	if len(pieces) != 3 {
		return "", httperror.NewAPIError(httperror.InvalidFormat, "invalid token")
	}

	decoded, err := base64.RawStdEncoding.DecodeString(pieces[1])
	if err != nil {
		return "", httperror.NewAPIError(httperror.InvalidFormat, "invalid token")
	}

	var dat map[string]interface{}

	err = json.Unmarshal([]byte(decoded), &dat)
	if err != nil {
		return "", httperror.NewAPIError(httperror.InvalidFormat, "invalid token")
	}

	if _, ok := dat[fieldID]; !ok {
		return "", httperror.NewAPIError(httperror.InvalidFormat, "invalid token")
	}
	return dat[fieldID].(string), nil
}
