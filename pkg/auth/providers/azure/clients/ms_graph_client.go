package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/manicminer/hamilton/environments"
	"github.com/manicminer/hamilton/msgraph"
	"github.com/manicminer/hamilton/odata"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AccessTokenSecretName is the name of the secret that contains an access token for the Microsoft Graph API.
	AccessTokenSecretName = "azuread-access-token"

	providerLogPrefix = "AZUREAD_PROVIDER"
	cacheLogPrefix    = "AZUREAD_PROVIDER_CACHE"
)

type azureMSGraphClient struct {
	authResult  *customAuthResult
	userClient  *msgraph.UsersClient
	groupClient *msgraph.GroupsClient
}

// MarshalTokenJSON returns the JSON representation of the underlying access token.
func (c azureMSGraphClient) MarshalTokenJSON() (string, error) {
	b, err := json.Marshal(c.authResult.AccessToken)
	return string(b), err
}

// GetUser takes a user ID and fetches the user principal from the Microsoft Graph API.
func (c azureMSGraphClient) GetUser(id string) (v3.Principal, error) {
	user, _, err := c.userClient.Get(context.Background(), id, odata.Query{})
	if err != nil {
		return v3.Principal{}, err
	}
	return c.userToPrincipal(*user)
}

// ListUsers fetches all user principals in a directory from the Microsoft Graph API.
func (c azureMSGraphClient) ListUsers(filter string) ([]v3.Principal, error) {
	users, _, err := c.userClient.List(context.Background(), odata.Query{Filter: filter})
	if err != nil {
		return nil, err
	}
	var principals []v3.Principal
	for _, u := range *users {
		principal, err := c.userToPrincipal(u)
		if err != nil {
			return nil, err
		}
		principals = append(principals, principal)
	}
	return principals, err
}

// GetGroup takes a group ID and fetches the group principal from the Microsoft Graph API.
func (c azureMSGraphClient) GetGroup(id string) (v3.Principal, error) {
	g, _, err := c.groupClient.Get(context.Background(), id, odata.Query{})
	if err != nil {
		return v3.Principal{}, err
	}
	return c.groupToPrincipal(*g)
}

// ListGroups fetches all group principals in a directory from the Microsoft Graph API.
func (c azureMSGraphClient) ListGroups(filter string) ([]v3.Principal, error) {
	groups, _, err := c.groupClient.List(context.Background(), odata.Query{Filter: filter})
	if err != nil {
		return nil, err
	}
	var principals []v3.Principal
	for _, u := range *groups {
		principal, err := c.groupToPrincipal(u)
		if err != nil {
			return nil, err
		}
		principals = append(principals, principal)
	}
	return principals, err
}

// ListGroupMemberships takes a user ID and fetches the user's group principals as string IDs from the Microsoft Graph API.
func (c azureMSGraphClient) ListGroupMemberships(id string) ([]string, error) {
	groups, _, err := c.userClient.ListGroupMemberships(context.Background(), id, odata.Query{})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, g := range *groups {
		if g.ID != nil && g.DisplayName != nil && g.SecurityEnabled != nil {
			names = append(names, *g.ID)
		}
	}
	return names, nil
}

// LoginUser verifies the user and fetches the user principal, user's group principals. It deliberately does not return
// the provider access token because the client itself handles its caching and does not need to return it.
func (c azureMSGraphClient) LoginUser(config *v32.AzureADConfig, credential *v32.AzureADLogin) (v3.Principal, []v3.Principal, string, error) {
	logrus.Debugf("[%s] Started token swap with AzureAD", providerLogPrefix)

	// Acquire the OID just to verify the user.
	oid, err := oidFromAuthCode(credential.Code, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	logrus.Debugf("[%s] Completed token swap with AzureAD", providerLogPrefix)

	logrus.Debugf("[%s] Started getting user info from AzureAD", providerLogPrefix)
	userPrincipal, err := c.GetUser(oid)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	userPrincipal.Me = true
	logrus.Debugf("[%s] Completed getting user info from AzureAD", providerLogPrefix)

	userGroups, err := c.ListGroupMemberships(GetPrincipalID(userPrincipal))
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	groupPrincipals, err := UserGroupsToPrincipals(c, userGroups)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	// Return an empty string for the provider token, so that it does not get saved in a secret later, like users'
	// access tokens are stored in secrets in the old Azure AD Graph flow.
	return userPrincipal, groupPrincipals, "", nil
}

type customAuthResult struct {
	AccessToken    string    `json:"accessToken,omitempty"`
	ExpiresOn      time.Time `json:"expiresOn,omitempty"`
	GrantedScopes  []string  `json:"grantedScopes,omitempty"`
	DeclinedScopes []string  `json:"declinedScopes,omitempty"`
}

// AccessToken returns the client's underlying provider access token.
func (c azureMSGraphClient) AccessToken() string {
	return c.authResult.AccessToken
}

type authorizer struct {
	authResult *customAuthResult
}

// AuxiliaryTokens is a no-op that satisfies the Authorizer interface for the SDK to the Microsoft Graph API.
func (a *authorizer) AuxiliaryTokens() ([]*oauth2.Token, error) {
	return nil, nil
}

// Token transforms the underlying provider access token into an OAuth2 token.
func (a *authorizer) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: a.authResult.AccessToken,
		TokenType:   "Bearer",
		Expiry:      a.authResult.ExpiresOn,
	}, nil
}

// oidFromAuthCode verifies a user.
func oidFromAuthCode(token string, config *v32.AzureADConfig) (string, error) {
	cred, err := confidential.NewCredFromSecret(config.ApplicationSecret)
	if err != nil {
		return "", fmt.Errorf("could not create a cred from a secret: %w", err)
	}
	confidentialClientApp, err := confidential.New(config.ApplicationID, cred, confidential.WithAuthority(fmt.Sprintf("%s%s", config.Endpoint, config.TenantID)))
	if err != nil {
		return "", err
	}
	scope := fmt.Sprintf("%s/%s", config.GraphEndpoint, ".default")
	authResult, err := confidentialClientApp.AcquireTokenByAuthCode(context.Background(), token, config.RancherURL, []string{scope})
	if err != nil {
		return "", err
	}

	return authResult.IDToken.Oid, nil
}

// AccessTokenCache is responsible for reading (replacing) the access token from some storage (a secret in the database,
// in this case) and writing it (exporting) to the storage.
// The Microsoft Graph SDK is responsible for verifying and calling the cache methods when needed,
// so this type simply implements the interface for the cache that the SDK requires.
// The SDK is also responsible for fetching a refresh token or a new access token, depending on how close expiration is.
// By default, a new access token from Microsoft Graph expires in one hour.
type AccessTokenCache struct {
	Secrets corev1.SecretInterface
}

// Replace fetches the access token from a secret in Kubernetes.
func (c AccessTokenCache) Replace(cache cache.Unmarshaler, key string) {
	secretName := fmt.Sprintf("%s:%s", common.SecretsNamespace, AccessTokenSecretName)
	secret, err := common.ReadFromSecret(c.Secrets, secretName, "access-token")
	if err != nil {
		logrus.Errorf("[%s] failed to read the access token from Kubernetes: %v", cacheLogPrefix, err)
		return
	}

	err = cache.Unmarshal([]byte(secret))
	if err != nil {
		logrus.Errorf("[%s] failed to unmarshal the access token: %v", cacheLogPrefix, err)
	}
}

// Export persists the access token to a secret in Kubernetes.
func (c AccessTokenCache) Export(cache cache.Marshaler, key string) {
	marshalled, err := cache.Marshal()
	if err != nil {
		logrus.Errorf("[%s] failed to marshal the access token before saving in Kubernetes: %v", cacheLogPrefix, err)
		return
	}

	err = common.CreateOrUpdateSecrets(c.Secrets, string(marshalled), "access-token", "azuread")
	if err != nil {
		logrus.Errorf("[%s] failed to save the access token in Kubernetes: %v", cacheLogPrefix, err)
	}
}

// NewMSGraphClient returns a client of the Microsoft Graph API. It attempts to get an access token to the API.
// It first tries to fetch the token from the refresh token, if the access token is found in the database.
// If that fails, it tries to acquire it directly from the auth provider with the credential (application secret in Azure).
// It also checks that the access token has the necessary permissions.
func NewMSGraphClient(config *v32.AzureADConfig, secrets corev1.SecretInterface) (AzureClient, error) {
	c := &azureMSGraphClient{}
	cred, err := confidential.NewCredFromSecret(config.ApplicationSecret)
	if err != nil {
		return nil, fmt.Errorf("could not create a cred from a secret: %w", err)
	}
	tokenCache := AccessTokenCache{Secrets: secrets}
	confidentialClientApp, err := confidential.New(config.ApplicationID, cred,
		confidential.WithAccessor(tokenCache),
		confidential.WithAuthority(fmt.Sprintf("%s%s", config.Endpoint, config.TenantID)))
	if err != nil {
		return nil, err
	}
	scope := fmt.Sprintf("%s/%s", config.GraphEndpoint, ".default")

	var ar confidential.AuthResult
	ar, err = confidentialClientApp.AcquireTokenSilent(context.Background(), []string{scope})
	if err != nil {
		logrus.Infof("failed to get the access token from cache: %v", err)
		logrus.Infoln("attempting to acquire the access token by credential")
		ar, err = confidentialClientApp.AcquireTokenByCredential(context.Background(), []string{scope})
		if err != nil {
			return nil, err
		}
	}

	authResult := getCustomAuthResult(&ar)
	authorizer := authorizer{authResult: authResult}

	userClient := msgraph.NewUsersClient(config.TenantID)
	userClient.BaseClient.Endpoint = environments.ApiEndpoint(config.GraphEndpoint)
	userClient.BaseClient.Authorizer = &authorizer
	userClient.BaseClient.ApiVersion = msgraph.Version10
	userClient.BaseClient.DisableRetries = true

	groupClient := msgraph.NewGroupsClient(config.TenantID)
	groupClient.BaseClient.Endpoint = environments.ApiEndpoint(config.GraphEndpoint)
	groupClient.BaseClient.Authorizer = &authorizer
	groupClient.BaseClient.ApiVersion = msgraph.Version10
	groupClient.BaseClient.DisableRetries = true

	c.authResult = authResult
	c.userClient = userClient
	c.groupClient = groupClient
	return c, err
}

func getCustomAuthResult(result *confidential.AuthResult) *customAuthResult {
	return &customAuthResult{
		AccessToken:    result.AccessToken,
		ExpiresOn:      result.ExpiresOn,
		GrantedScopes:  result.GrantedScopes,
		DeclinedScopes: result.DeclinedScopes,
	}
}

func (c azureMSGraphClient) userToPrincipal(user msgraph.User) (v3.Principal, error) {
	return v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: Name + "_user://" + *user.ID},
		DisplayName:   *user.DisplayName,
		LoginName:     *user.UserPrincipalName,
		PrincipalType: "user",
		Provider:      Name,
	}, nil
}

func (c azureMSGraphClient) groupToPrincipal(group msgraph.Group) (v3.Principal, error) {
	return v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: Name + "_group://" + *group.ID},
		DisplayName:   *group.DisplayName,
		PrincipalType: "group",
		Provider:      Name,
	}, nil
}
