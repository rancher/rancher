package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// Name of the provider
	Name = "azuread"
)

type azureProvider struct {
	ctx         context.Context
	authConfigs v3.AuthConfigInterface
	userMGR     user.Manager
	tokenMGR    *tokens.Manager
}

func Configure(
	ctx context.Context,
	mgmtCtx *config.ScaledContext,
	userMGR user.Manager,
	tokenMGR *tokens.Manager,
) common.AuthProvider {

	return &azureProvider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
	}
}

func (ap *azureProvider) GetName() string {
	return Name
}

func (ap *azureProvider) AuthenticateUser(
	input interface{},
) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v3public.AzureADLogin)
	if !ok {
		return v3.Principal{}, nil, "", errors.New("unexpected input type")
	}
	return ap.loginUser(login, nil, false)
}

func (ap *azureProvider) SearchPrincipals(
	name string,
	principalType string,
	token v3.Token,
) ([]v3.Principal, error) {
	var princ []v3.Principal

	client, err := ap.newAzureClient(token)
	if err != nil {
		return nil, err
	}

	switch principalType {
	case "user":
		princ, err = ap.searchUsers(client, name, token)
		if err != nil {
			return nil, err
		}
	case "group":
		princ, err = ap.searchGroups(client, name, token)
		if err != nil {
			return nil, err
		}
	case "":
		users, err := ap.searchUsers(client, name, token)
		if err != nil {
			return nil, err
		}
		groups, err := ap.searchGroups(client, name, token)
		if err != nil {
			return nil, err
		}
		princ = append(princ, users...)
		princ = append(princ, groups...)

	}

	return princ, ap.updateToken(client, &token)
}

func (ap *azureProvider) GetPrincipal(
	principalID string,
	token v3.Token,
) (v3.Principal, error) {
	var princ v3.Principal
	var err error

	client, err := ap.newAzureClient(token)
	if err != nil {
		return princ, err
	}

	parsed, err := parsePrincipalID(principalID)
	if err != nil {
		return v3.Principal{}, httperror.NewAPIError(httperror.NotFound, "invalid principal")
	}

	switch parsed["type"] {
	case "user":
		princ, err = ap.getUser(client, parsed["ID"], token)
	case "group":
		princ, err = ap.getGroup(client, parsed["ID"], token)
	}

	if err != nil {
		return v3.Principal{}, err
	}

	return princ, ap.updateToken(client, &token)

}

func (ap *azureProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = ap.actionHandler
	schema.Formatter = ap.formatter
}

func (ap *azureProvider) TransformToAuthProvider(
	authConfig map[string]interface{},
) map[string]interface{} {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.AzureADProviderFieldRedirectURL] = formAzureRedirectURL(authConfig)
	return p
}

func (ap *azureProvider) loginUser(
	azureCredential *v3public.AzureADLogin,
	config *v3.AzureADConfig,
	test bool,
) (v3.Principal, []v3.Principal, string, error) {
	var err error

	if config == nil {
		config, err = ap.getAzureConfigK8s()
		if err != nil {
			return v3.Principal{}, nil, "", err
		}
	}

	azureClient, err := newClientCode(azureCredential.Code, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	oid, err := parseJWTforField(azureClient.accessToken(), "oid")
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	user, err := azureClient.userClient.Get(context.Background(), oid)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	userPrincipal := ap.userToPrincipal(user)
	userPrincipal.Me = true

	var mem bool
	params := graphrbac.UserGetMemberGroupsParameters{
		SecurityEnabledOnly: &mem,
	}

	userGroups, err := azureClient.userClient.GetMemberGroups(context.Background(), *user.ObjectID, params)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	groupPrincipals, err := ap.userGroupsToPrincipals(azureClient, userGroups)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	testAllowedPrincipals := config.AllowedPrincipalIDs
	if test && config.AccessMode == "restricted" {
		testAllowedPrincipals = append(testAllowedPrincipals, userPrincipal.Name)
	}

	allowed, err := ap.userMGR.CheckAccess(config.AccessMode, testAllowedPrincipals, userPrincipal, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	if !allowed {
		return v3.Principal{}, nil, "", httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	providerToken, err := azureClient.marshalTokenJSON()
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	return userPrincipal, groupPrincipals, string(providerToken), nil
}

func (ap *azureProvider) getUser(client *azureClient, principalID string, token v3.Token) (v3.Principal, error) {
	user, err := client.userClient.Get(context.Background(), principalID)
	if err != nil {
		return v3.Principal{}, httperror.NewAPIError(httperror.NotFound, err.Error())
	}

	p := ap.userToPrincipal(user)
	p.Me = samePrincipal(token.UserPrincipal, p)

	return p, nil
}

func (ap *azureProvider) getGroup(client *azureClient, principalID string, token v3.Token) (v3.Principal, error) {
	group, err := client.groupClient.Get(context.Background(), principalID)
	if err != nil {
		return v3.Principal{}, httperror.NewAPIError(httperror.NotFound, err.Error())
	}

	p := ap.groupToPrincipal(group)
	p.MemberOf = ap.tokenMGR.IsMemberOf(token, p)

	return p, nil
}

func (ap *azureProvider) searchUsers(client *azureClient, name string, token v3.Token) ([]v3.Principal, error) {
	filter := fmt.Sprintf("startswith(userPrincipalName,'%[1]s') or startswith(displayName,'%[1]s') or startswith(givenName,'%[1]s') or startswith(surname,'%[1]s')", name)
	users, err := client.userClient.List(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var principals []v3.Principal
	for _, user := range users.Values() {
		p := ap.userToPrincipal(user)
		p.Me = samePrincipal(token.UserPrincipal, p)
		principals = append(principals, p)
	}
	return principals, nil
}

func (ap *azureProvider) searchGroups(client *azureClient, name string, token v3.Token) ([]v3.Principal, error) {
	filter := fmt.Sprintf("startswith(displayName,'%[1]s') or startswith(mail,'%[1]s') or startswith(mailNickname,'%[1]s')", name)
	groups, err := client.groupClient.List(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var principals []v3.Principal
	for _, group := range groups.Values() {
		p := ap.groupToPrincipal(group)
		p.MemberOf = ap.tokenMGR.IsMemberOf(token, p)
		principals = append(principals, p)
	}
	return principals, nil
}

func (ap *azureProvider) newAzureClient(token v3.Token) (*azureClient, error) {
	config, err := ap.getAzureConfigK8s()
	if err != nil {
		return nil, err
	}

	azureToken, err := ap.getAzureToken(&token)
	if err != nil {
		return nil, err
	}

	client, err := newClientToken(token, config, azureToken)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (ap *azureProvider) saveAzureConfigK8s(config *v3.AzureADConfig) error {
	storedAzureConfig, err := ap.getAzureConfigK8s()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.AzureADConfigType
	config.ObjectMeta = storedAzureConfig.ObjectMeta

	logrus.Debugf("updating AzureADConfig")
	_, err = ap.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (ap *azureProvider) getAzureConfigK8s() (*v3.AzureADConfig, error) {
	authConfigObj, err := ap.authConfigs.ObjectClient().UnstructuredClient().Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, cannot read k8s Unstructured data")
	}
	storedAzureADConfigMap := u.UnstructuredContent()

	storedAzureADConfig := &v3.AzureADConfig{}
	mapstructure.Decode(storedAzureADConfigMap, storedAzureADConfig)

	metadataMap, ok := storedAzureADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig metadata, cannot read k8s Unstructured data")
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedAzureADConfig.ObjectMeta = *objectMeta

	return storedAzureADConfig, nil
}

func (ap *azureProvider) userToPrincipal(user graphrbac.User) v3.Principal {
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: Name + "_user://" + *user.ObjectID},
		DisplayName:   *user.DisplayName,
		LoginName:     *user.UserPrincipalName,
		PrincipalType: "user",
		Provider:      Name,
	}
	return p
}

func (ap *azureProvider) userGroupsToPrincipals(
	azureClient *azureClient,
	groups graphrbac.UserGetMemberGroupsResult,
) ([]v3.Principal, error) {
	var groupPrincipals []v3.Principal
	for _, group := range *groups.Value {
		groupObj, err := azureClient.groupClient.Get(context.Background(), group)
		if err != nil {
			return groupPrincipals, err
		}

		p := ap.groupToPrincipal(groupObj)
		p.MemberOf = true

		groupPrincipals = append(groupPrincipals, p)
	}
	return groupPrincipals, nil

}

func (ap *azureProvider) groupToPrincipal(group graphrbac.ADGroup) v3.Principal {
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: Name + "_group://" + *group.ObjectID},
		DisplayName:   *group.DisplayName,
		PrincipalType: "group",
		Provider:      Name,
	}
	return p
}

func (ap *azureProvider) getAzureToken(token *v3.Token) (adal.Token, error) {
	var azureToken adal.Token
	secret, err := ap.tokenMGR.GetSecret(token)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return azureToken, err
		}
		secret = token.ProviderInfo["access_token"]
	}
	err = json.Unmarshal([]byte(secret), &azureToken)
	if err != nil {
		return azureToken, err
	}
	return azureToken, nil
}

// updateToken compares the current azure token to the azure token living in the
// secret and updates if needed
func (ap *azureProvider) updateToken(client *azureClient, token *v3.Token) error {
	new, err := client.marshalTokenJSON()
	if err != nil {
		return err
	}
	stringNew := string(new)

	secret, err := ap.tokenMGR.GetSecret(token)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// providerToken doesn't exists as a secret, update on token
			if current, ok := token.ProviderInfo["access_token"]; ok && current != stringNew {
				token.ProviderInfo["access_token"] = stringNew
			}
			return nil
		}
		return err
	}

	if stringNew == secret {
		return nil
	}

	return ap.tokenMGR.UpdateSecret(token.UserID, token.AuthProvider, stringNew)
}

func formAzureRedirectURL(config map[string]interface{}) string {
	return fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&resource=%s",
		config["authEndpoint"],
		config["applicationId"],
		config["rancherUrl"],
		config["graphEndpoint"],
	)
}

// parsePrincipalID accepts a principalID in the format <provider>_<type>://<ID>
// and returns a map of the 3 parts - ID, provider and type
func parsePrincipalID(principalID string) (map[string]string, error) {
	parsed := make(map[string]string)

	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return parsed, errors.Errorf("invalid id %v", principalID)
	}
	externalID := strings.TrimPrefix(parts[1], "//")

	parsed["ID"] = externalID

	pparts := strings.SplitN(parts[0], "_", 2)
	if len(pparts) != 2 {
		return parsed, errors.Errorf("invalid id %v", principalID)
	}

	parsed["provider"] = pparts[0]
	parsed["type"] = pparts[1]

	return parsed, nil
}

// samePrincipal verifies if they are the same principal
func samePrincipal(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}
