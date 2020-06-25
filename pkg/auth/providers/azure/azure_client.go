package azure

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
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
func newClientToken(config *v3.AzureADConfig, azureToken adal.Token) (*azureClient, error) {
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
	logrus.Infof("Obtained token: %v", tokenString)
	logrus.Infof("Splitting Azure AD access token")
	pieces := strings.Split(tokenString, ".")
	if len(pieces) != 3 {
		logrus.Errorf("Incorrect Azure AD token length")
		return "", httperror.NewAPIError(httperror.InvalidFormat, "incorrect azure AD token length")
	}
	logrus.Infof("Decoding token part %v", pieces[1])
	logrus.Infof("Decoding Azure AD access token")
	decoded, err := base64.RawURLEncoding.DecodeString(pieces[1])
	if err != nil {
		logrus.Errorf("Error decoding Azure AD token: %v", err)
		return "", httperror.NewAPIError(httperror.InvalidFormat, "error decoding azure AD token")
	}

	var dat map[string]interface{}
	logrus.Infof("Unmarshaling decoded Azure AD access token %v", decoded)
	err = json.Unmarshal([]byte(decoded), &dat)
	if err != nil {
		logrus.Errorf("Error unmarshaling Azure AD decoded token: %v", err)
		return "", httperror.NewAPIError(httperror.InvalidFormat, "error unmarshaling azure AD token")
	}
	logrus.Infof("Retrieved field oid after unmarshal from: %v", dat)
	if _, ok := dat[fieldID]; !ok {
		logrus.Errorf("No value for oid passed in token")
		return "", httperror.NewAPIError(httperror.InvalidFormat, "error retrieving oid from azure AD token")
	}
	return dat[fieldID].(string), nil
}
