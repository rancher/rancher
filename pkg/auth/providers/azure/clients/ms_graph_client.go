package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/coreos/go-oidc/v3/oidc"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	msgraphgroups "github.com/microsoftgraph/msgraph-sdk-go/groups"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
	msgraphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	normancorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AccessTokenSecretName is the name of the secret that contains an access token for the Microsoft Graph API.
	AccessTokenSecretName = "azuread-access-token"

	providerLogPrefix = "AZUREAD_PROVIDER"
	cacheLogPrefix    = "AZUREAD_PROVIDER_CACHE"
)

// NewMSGraphClient creates and returns a new client for accessing the Azure
// Graph client.

// It first tries to fetch the token from the refresh token, if the access token is found in the database.
// If that fails, it tries to acquire it directly from the auth provider with the credential (application secret in Azure).
// It also checks that the access token has the necessary permissions.
func NewMSGraphClient(config *v32.AzureADConfig, secrets normancorev1.SecretInterface) (*AzureMSGraphClient, error) {
	cred, err := confidential.NewCredFromSecret(config.ApplicationSecret)
	if err != nil {
		return nil, fmt.Errorf("could not create a cred from a secret: %w", err)
	}

	authorityURL, err := url.JoinPath(config.Endpoint, config.TenantID)
	if err != nil {
		return nil, fmt.Errorf("could not create token authority url: %w", err)
	}

	tokenCache := accessTokenCache{Secrets: secrets}
	confidentialClient, err := confidential.New(authorityURL, config.ApplicationID, cred,
		confidential.WithCache(tokenCache))
	if err != nil {
		return nil, fmt.Errorf("creating MS Graph client: %w", err)
	}

	graphEndpoint, err := url.JoinPath(config.GraphEndpoint, ".default")
	if err != nil {
		return nil, fmt.Errorf("making graph endpoint for %s: %w", config.GraphEndpoint, err)
	}

	var ar confidential.AuthResult
	ctx := context.Background()
	ar, err = confidentialClient.AcquireTokenSilent(ctx, []string{graphEndpoint})
	if err != nil {
		logrus.Debugf("[%s] failed to get the access token from cache: %s", providerLogPrefix, err)
		logrus.Debugf("[%s] attempting to acquire the access token by credential", providerLogPrefix)
		ar, err = confidentialClient.AcquireTokenByCredential(ctx, []string{graphEndpoint})
		if err != nil {
			return nil, fmt.Errorf("acquiring token by credential: %w", err)
		}
	} else {
		logrus.Debugf("[%s] acquired token from cache", providerLogPrefix)
	}

	authResult := getCustomAuthResult(ar)

	logrus.Debugf("[%s] connecting to graph endpoint: %s", providerLogPrefix, graphEndpoint)
	graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(
		authResult, []string{graphEndpoint})
	if err != nil {
		return nil, fmt.Errorf("creating graph service client: %w", err)
	}

	graphBaseURL, err := url.JoinPath(config.GraphEndpoint, "v1.0")
	if err != nil {
		return nil, fmt.Errorf("making graph base url for %s: %w", config.GraphEndpoint, err)
	}
	logrus.Debugf("[%s] graph base url: %s", providerLogPrefix, graphBaseURL)
	graphClient.GetAdapter().SetBaseUrl(graphBaseURL)

	return &AzureMSGraphClient{
		Credential:         cred,
		GraphEndpointURL:   graphEndpoint,
		GraphClient:        graphClient,
		ConfidentialClient: confidentialClient,
		authResult:         authResult,
	}, nil
}

// AzureMSGraphClient queries the Azure graph API to get users and groups.
type AzureMSGraphClient struct {
	confidential.Credential
	GraphEndpointURL   string
	authResult         *customAuthResult
	ConfidentialClient confidential.Client

	GraphClient *msgraphsdkgo.GraphServiceClient
}

// GetUser takes a user ID and fetches the user principal from the Microsoft Graph API.
func (c AzureMSGraphClient) GetUser(userID string) (v3.Principal, error) {
	logrus.Debugf("[%s] GetUser %s", providerLogPrefix, userID)
	result, err := c.GraphClient.Users().ByUserId(userID).Get(context.Background(), nil)
	if err != nil {
		return v3.Principal{}, fmt.Errorf("getting user by ID: %w", getMSGraphErrorData(err))
	}

	return userToPrincipal(result), nil
}

// ListUsers fetches all user principals in a directory from the Microsoft Graph API.
func (c AzureMSGraphClient) ListUsers(filter string) ([]v3.Principal, error) {
	logrus.Debugf("[%s] ListUsers %s", providerLogPrefix, filter)
	result, err := c.GraphClient.Users().Get(context.Background(), &msgraphusers.UsersRequestBuilderGetRequestConfiguration{
		QueryParameters: &msgraphusers.UsersRequestBuilderGetQueryParameters{
			Filter: &filter,
		}})
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", getMSGraphErrorData(err))
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Userable](
		result, c.GraphClient.GetAdapter(), models.CreateUserCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, fmt.Errorf("iterating over user list: %w", getMSGraphErrorData(err))
	}

	var users []v3.Principal
	err = pageIterator.Iterate(context.Background(), func(user models.Userable) bool {
		users = append(users, userToPrincipal(user))
		return true
	})

	return users, err
}

// GetGroup takes a group ID and fetches the group principal from the Microsoft Graph API.
func (c AzureMSGraphClient) GetGroup(groupID string) (v3.Principal, error) {
	logrus.Debugf("[%s] GetGroup %s", providerLogPrefix, groupID)
	result, err := c.GraphClient.Groups().ByGroupId(groupID).Get(context.Background(), nil)
	if err != nil {
		return v3.Principal{}, fmt.Errorf("getting group by ID: %w", getMSGraphErrorData(err))
	}

	return groupToPrincipal(result), nil
}

// ListGroups fetches all group principals in a directory from the Microsoft Graph API.
func (c AzureMSGraphClient) ListGroups(filter string) ([]v3.Principal, error) {
	logrus.Debugf("[%s] ListGroups %s", providerLogPrefix, filter)
	result, err := c.GraphClient.Groups().Get(context.Background(), &msgraphgroups.GroupsRequestBuilderGetRequestConfiguration{
		QueryParameters: &msgraphgroups.GroupsRequestBuilderGetQueryParameters{
			Filter: &filter,
		}})
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", getMSGraphErrorData(err))
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Groupable](
		result, c.GraphClient.GetAdapter(), models.CreateGroupCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, fmt.Errorf("iterating over group list: %w", getMSGraphErrorData(err))
	}

	var groups []v3.Principal
	err = pageIterator.Iterate(context.Background(), func(group models.Groupable) bool {
		groups = append(groups, groupToPrincipal(group))
		return true
	})

	return groups, err

}

// ListGroupMemberships takes a user ID and fetches the user's group principals as string IDs from the Microsoft Graph API.
func (c AzureMSGraphClient) ListGroupMemberships(userID string, filter string) ([]string, error) {
	logrus.Debugf("[%s] ListGroupMemberships %s", providerLogPrefix, userID)
	var groupIDs []string

	err := c.listGroupMemberships(context.Background(), userID, filter, func(g *models.Group) {
		if id := g.GetId(); id != nil {
			groupIDs = append(groupIDs, *id)
		}
	})
	if err != nil {
		return nil, err
	}

	return groupIDs, nil
}

func (c AzureMSGraphClient) listGroupMemberships(ctx context.Context, userID string, filter string, f func(*models.Group)) error {
	requestCount := true
	headers := abstractions.NewRequestHeaders()
	headers.Add("ConsistencyLevel", "eventual")

	result, err := c.GraphClient.Users().
		ByUserId(userID).
		TransitiveMemberOf().
		Get(ctx,
			&msgraphusers.ItemTransitiveMemberOfRequestBuilderGetRequestConfiguration{
				Headers: headers,
				QueryParameters: &msgraphusers.ItemTransitiveMemberOfRequestBuilderGetQueryParameters{
					Filter: &filter,
					Count:  &requestCount,
				}})
	if err != nil {
		return fmt.Errorf("listing group memberships: %w", getMSGraphErrorData(err))
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.DirectoryObjectable](
		result, c.GraphClient.GetAdapter(),
		models.CreateDirectoryObjectCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return fmt.Errorf("iterating over group membership list: %w", getMSGraphErrorData(err))
	}

	err = pageIterator.Iterate(ctx, func(do models.DirectoryObjectable) bool {
		group, ok := do.(*models.Group)
		if !ok {
			if _, ok := do.(*models.DirectoryRole); !ok {
				logrus.Errorf("[%s] Page Iterator received incorrect value of type %T: %#v", providerLogPrefix, do, do)
			}
			return true
		}
		f(group)

		return true
	})

	return err
}

// LoginUser verifies the user and fetches the user principal, user's group principals. It deliberately does not return
// the provider access token because the client itself handles its caching and does not need to return it.
func (c AzureMSGraphClient) LoginUser(config *v32.AzureADConfig, credential *v32.AzureADLogin) (v3.Principal, []v3.Principal, string, error) {
	logrus.Debugf("[%s] Started token swap with AzureAD", providerLogPrefix)

	oid, err := c.getOIDFromLogin(config, credential)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	logrus.Debugf("[%s] Completed token swap with AzureAD", providerLogPrefix)

	logrus.Debugf("[%s] Started getting user info from AzureAD", providerLogPrefix)
	userPrincipal, err := c.GetUser(oid)
	if err != nil {
		return v3.Principal{}, nil, "", fmt.Errorf("getting UserInfo from Azure: %w", err)
	}
	userPrincipal.Me = true
	logrus.Debugf("[%s] Completed getting user info from AzureAD", providerLogPrefix)

	groupPrincipals, err := c.listGroupPrincipals(context.Background(), userPrincipal, config.GroupMembershipFilter)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	return userPrincipal, groupPrincipals, "", nil
}

func (c AzureMSGraphClient) listGroupPrincipals(ctx context.Context, userPrincipal v3.Principal, filter string) ([]v3.Principal, error) {
	var groups []string
	err := c.listGroupMemberships(ctx, GetPrincipalID(userPrincipal), filter, func(g *models.Group) {
		if id := g.GetId(); id != nil && g.GetDisplayName() != nil && g.GetSecurityEnabled() != nil {
			groups = append(groups, *id)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("listing group memberships: %w", err)
	}

	groupPrincipals, err := UserGroupsToPrincipals(c, groups)
	if err != nil {
		return nil, fmt.Errorf("converting groups to principals: %w", err)
	}

	return groupPrincipals, nil
}

func (c AzureMSGraphClient) getOIDFromLogin(config *v32.AzureADConfig, credential *v32.AzureADLogin) (string, error) {
	if credential.IDToken != "" {
		// Acquire the OID from the IDToken to verify the user
		oidFromToken, err := oidFromIDToken(credential.IDToken, config)
		if err != nil {
			return "", fmt.Errorf("getting OID from IDToken: %w", err)
		}

		return oidFromToken, nil
	}

	// Acquire the OID exchanging the Code to verify the user
	oidFromCode, err := oidFromAuthCode(credential.Code, config, c.GraphEndpointURL)
	if err != nil {
		return "", fmt.Errorf("getting OID from AuthCode: %w", err)
	}

	return oidFromCode, nil
}

// AccessToken returns the client's underlying provider access token.
func (c AzureMSGraphClient) AccessToken() string {
	return c.authResult.AccessToken
}

// MarshalTokenJSON returns the JSON representation of the underlying access token.
func (c AzureMSGraphClient) MarshalTokenJSON() (string, error) {
	b, err := json.Marshal(c.authResult.AccessToken)

	return string(b), err
}

func userToPrincipal(user models.Userable) v3.Principal {
	return v3.Principal{
		ObjectMeta: metav1.ObjectMeta{
			Name: Name + "_user://" + *user.GetId(),
		},
		DisplayName:   *user.GetDisplayName(),
		LoginName:     *user.GetUserPrincipalName(),
		PrincipalType: "user",
		Provider:      Name,
	}
}

func groupToPrincipal(group models.Groupable) v3.Principal {
	return v3.Principal{
		ObjectMeta: metav1.ObjectMeta{
			Name: Name + "_group://" + *group.GetId(),
		},
		DisplayName:   *group.GetDisplayName(),
		PrincipalType: "group",
		Provider:      Name,
	}
}

// oidFromIDToken verifies the IDToken, returning the user OID
func oidFromIDToken(token string, config *v32.AzureADConfig) (string, error) {
	issuer, err := url.JoinPath(config.Endpoint, config.TenantID, "/v2.0")
	if err != nil {
		return "", fmt.Errorf("joining issuer path: %w", err)
	}

	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return "", fmt.Errorf("creating OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: config.ApplicationID})
	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return "", fmt.Errorf("verifying user ID Token: %w", err)
	}

	var claims struct {
		OID string `json:"oid"`
	}

	if err = idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("extracting claims: %w", err)
	}

	if claims.OID == "" {
		return "", errors.New("empty user OID")
	}

	return claims.OID, nil
}

// oidFromAuthCode exchanges the AuthCode for a IDToken, returning the user OID
func oidFromAuthCode(token string, config *v32.AzureADConfig, endpointURL string) (string, error) {
	cred, err := confidential.NewCredFromSecret(config.ApplicationSecret)
	if err != nil {
		return "", fmt.Errorf("could not create a cred from a secret: %w", err)
	}
	authorityURL, err := url.JoinPath(config.Endpoint, config.TenantID)
	if err != nil {
		return "", fmt.Errorf("could not create token authority url: %w", err)
	}

	// NOTE: This uses a new client which is not associated to a token cache,
	// this means that the token is never cached (and no user tokens are cached)
	// this keeps the cache-size down and improves security.
	confidentialClientApp, err := confidential.New(authorityURL, config.ApplicationID, cred)
	if err != nil {
		return "", err
	}

	authResult, err := confidentialClientApp.AcquireTokenByAuthCode(context.Background(), token, config.RancherURL, []string{endpointURL})
	if err != nil {
		return "", err
	}

	return authResult.IDToken.Oid, nil
}

func getMSGraphErrorData(err error) error {
	if odataErr, ok := err.(*odataerrors.ODataError); ok {
		if oErr := odataErr.GetErrorEscaped(); oErr != nil {
			return fmt.Errorf("%v: code: %s, msg: %s", odataErr, *oErr.GetCode(), *oErr.GetMessage())
		}
		return odataErr
	}
	return err
}

// accessTokenCache is responsible for reading (replacing) the access token from some storage (a secret in the database,
// in this case) and writing it (exporting) to the storage.
// The Microsoft Graph SDK is responsible for verifying and calling the cache methods when needed,
// so this type simply implements the interface for the cache that the SDK requires.
// The SDK is also responsible for fetching a refresh token or a new access token, depending on how close expiration is.
// By default, a new access token from Microsoft Graph expires in one hour.
//
// WARNING: The tokens are stored in plain-text in Kubernetes secrets.
type accessTokenCache struct {
	Secrets normancorev1.SecretInterface
}

// Replace fetches the access token from a secret in Kubernetes.
func (c accessTokenCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	secretName := fmt.Sprintf("%s:%s", common.SecretsNamespace, AccessTokenSecretName)
	secret, err := common.ReadFromSecret(c.Secrets, secretName, "access-token")
	if err != nil {
		logrus.Errorf("[%s] Failed to read the access token from Kubernetes: %v", cacheLogPrefix, err)
		return client.IgnoreNotFound(err)
	}

	err = cache.Unmarshal([]byte(secret))
	if err != nil {
		logrus.Errorf("[%s] Failed to unmarshal the access token: %v", cacheLogPrefix, err)
		return err
	}

	return nil
}

// Export persists the access token to a secret in Kubernetes.
func (c accessTokenCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	marshalled, err := cache.Marshal()
	if err != nil {
		logrus.Errorf("[%s] Failed to marshal the access token before saving in Kubernetes: %v", cacheLogPrefix, err)
		return err
	}

	_, err = common.CreateOrUpdateSecrets(c.Secrets, string(marshalled), "access-token", "azuread")
	if err != nil {
		logrus.Errorf("[%s] Failed to save the access token in Kubernetes: %v", cacheLogPrefix, err)
		return err
	}

	return nil
}

func getCustomAuthResult(result confidential.AuthResult) *customAuthResult {
	return &customAuthResult{
		AccessToken:    result.AccessToken,
		ExpiresOn:      result.ExpiresOn,
		GrantedScopes:  result.GrantedScopes,
		DeclinedScopes: result.DeclinedScopes,
	}
}

type customAuthResult struct {
	AccessToken    string    `json:"accessToken,omitempty"`
	ExpiresOn      time.Time `json:"expiresOn,omitempty"`
	GrantedScopes  []string  `json:"grantedScopes,omitempty"`
	DeclinedScopes []string  `json:"declinedScopes,omitempty"`
}

func (c *customAuthResult) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     c.AccessToken,
		ExpiresOn: c.ExpiresOn,
	}, nil
}
