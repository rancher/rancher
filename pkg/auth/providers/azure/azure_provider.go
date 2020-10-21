package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest/adal"
	lru "github.com/hashicorp/golang-lru"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// Name of the provider
	Name = "azuread"
)

var groupCache *lru.Cache

type azureProvider struct {
	ctx         context.Context
	authConfigs v3.AuthConfigInterface
	secrets     corev1.SecretInterface
	userMGR     user.Manager
	tokenMGR    *tokens.Manager
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	var err error
	groupCache, err = lru.New(settings.AzureGroupCacheSize.GetInt())
	if err != nil {
		logrus.Warnf("initial azure-group-cache-size was invalid value, setting to 10000 error:%v", err)
		groupCache, _ = lru.New(10000)
	}

	return &azureProvider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		secrets:     mgmtCtx.Core.Secrets(""),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
	}
}

func (ap *azureProvider) GetName() string {
	return Name
}

func (ap *azureProvider) AuthenticateUser(
	ctx context.Context, input interface{},
) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v32.AzureADLogin)
	if !ok {
		return v3.Principal{}, nil, "", errors.New("unexpected input type")
	}
	return ap.loginUser(login, nil, false)
}

func (ap *azureProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	azureClient, err := ap.newAzureClient(secret)
	if err != nil {
		return nil, err
	}

	oid, err := parseJWTforField(azureClient.accessToken(), "oid")
	if err != nil {
		return nil, err
	}

	logrus.Debug("[AZURE_PROVIDER] Started getting user info from AzureAD")

	user, err := azureClient.userClient.Get(context.Background(), oid)
	if err != nil {
		return nil, err
	}

	logrus.Debug("[AZURE_PROVIDER] Completed getting user info from AzureAD")

	var mem bool
	params := graphrbac.UserGetMemberGroupsParameters{
		SecurityEnabledOnly: &mem,
	}

	userGroups, err := azureClient.userClient.GetMemberGroups(context.Background(), *user.ObjectID, params)
	if err != nil {
		return nil, err
	}

	groupPrincipals, err := ap.userGroupsToPrincipals(azureClient, userGroups)
	if err != nil {
		return nil, err
	}

	return groupPrincipals, nil
}

func (ap *azureProvider) SearchPrincipals(
	name string,
	principalType string,
	token v3.Token,
) ([]v3.Principal, error) {
	var princ []v3.Principal

	secret, err := ap.tokenMGR.GetSecret(token.UserID, Name, []*v3.Token{&token})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	client, err := ap.newAzureClient(secret)
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

	secret, err := ap.tokenMGR.GetSecret(token.UserID, Name, []*v3.Token{&token})
	if err != nil && !apierrors.IsNotFound(err) {
		return princ, err
	}

	client, err := ap.newAzureClient(secret)
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
) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.AzureADProviderFieldRedirectURL] = formAzureRedirectURL(authConfig)
	return p, nil
}

func (ap *azureProvider) loginUser(
	azureCredential *v32.AzureADLogin,
	config *v32.AzureADConfig,
	test bool,
) (v3.Principal, []v3.Principal, string, error) {
	var err error

	if config == nil {
		config, err = ap.getAzureConfigK8s()
		if err != nil {
			return v3.Principal{}, nil, "", err
		}
	}

	logrus.Debug("[AZURE_PROVIDER] Started token swap with AzureAD")

	azureClient, err := newClientCode(azureCredential.Code, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	logrus.Debug("[AZURE_PROVIDER] Completed token swap with AzureAD")

	oid, err := parseJWTforField(azureClient.accessToken(), "oid")
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	logrus.Debug("[AZURE_PROVIDER] Started getting user info from AzureAD")

	user, err := azureClient.userClient.Get(context.Background(), oid)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	logrus.Debug("[AZURE_PROVIDER] Completed getting user info from AzureAD")

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

	allowed, err := ap.userMGR.CheckAccess(config.AccessMode, testAllowedPrincipals, userPrincipal.Name, groupPrincipals)
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
	users, err := client.userClient.List(context.Background(), filter, "")
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

func (ap *azureProvider) newAzureClient(secret string) (*azureClient, error) {
	config, err := ap.getAzureConfigK8s()
	if err != nil {
		return nil, err
	}

	azureToken, err := ap.getAzureToken(secret)
	if err != nil {
		return nil, err
	}

	client, err := newClientToken(config, azureToken)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (ap *azureProvider) saveAzureConfigK8s(config *v32.AzureADConfig) error {
	storedAzureConfig, err := ap.getAzureConfigK8s()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.AzureADConfigType
	config.ObjectMeta = storedAzureConfig.ObjectMeta

	field := strings.ToLower(client.AzureADConfigFieldApplicationSecret)
	if err := common.CreateOrUpdateSecrets(ap.secrets, config.ApplicationSecret, field, strings.ToLower(config.Type)); err != nil {
		return err
	}

	config.ApplicationSecret = common.GetName(config.Type, field)

	logrus.Debugf("updating AzureADConfig")
	_, err = ap.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (ap *azureProvider) getAzureConfigK8s() (*v32.AzureADConfig, error) {
	authConfigObj, err := ap.authConfigs.ObjectClient().UnstructuredClient().Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, cannot read k8s Unstructured data")
	}
	storedAzureADConfigMap := u.UnstructuredContent()

	storedAzureADConfig := &v32.AzureADConfig{}
	mapstructure.Decode(storedAzureADConfigMap, storedAzureADConfig)

	metadataMap, ok := storedAzureADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig metadata, cannot read k8s Unstructured data")
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedAzureADConfig.ObjectMeta = *objectMeta

	if storedAzureADConfig.ApplicationSecret != "" {
		value, err := common.ReadFromSecret(ap.secrets, storedAzureADConfig.ApplicationSecret,
			strings.ToLower(client.AzureADConfigFieldApplicationSecret))
		if err != nil {
			return nil, err
		}
		storedAzureADConfig.ApplicationSecret = value
	}

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

func (ap *azureProvider) userGroupsToPrincipals(azureClient *azureClient, groups graphrbac.UserGetMemberGroupsResult) ([]v3.Principal, error) {
	start := time.Now()
	logrus.Debug("[AZURE_PROVIDER] Started gathering users groups")

	var g errgroup.Group
	groupPrincipals := make([]v3.Principal, len(*groups.Value))
	for i, group := range *groups.Value {
		j := i
		gp := group

		// Check the cache first, if it exists that saves an API call to azure
		if principal, ok := groupCache.Get(gp); ok {
			p := principal.(v3.Principal)
			groupPrincipals[j] = p
			continue
		}

		g.Go(func() error {
			groupObj, err := azureClient.groupClient.Get(context.Background(), gp)
			if err != nil {
				logrus.Debugf("[AZURE_PROVIDER] Error getting group: %v", err)
				return err
			}

			p := ap.groupToPrincipal(groupObj)
			p.MemberOf = true

			// Add to the cache
			groupCache.Add(gp, p)
			groupPrincipals[j] = p
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	logrus.Debugf("[AZURE_PROVIDER] Completed gathering users groups, took %v, keys in cache:%v", time.Since(start), groupCache.Len())
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

func (ap *azureProvider) getAzureToken(secret string) (adal.Token, error) {
	var azureToken adal.Token
	err := json.Unmarshal([]byte(secret), &azureToken)
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

	secret, err := ap.tokenMGR.GetSecret(token.UserID, token.AuthProvider, []*v3.Token{token})
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

func (ap *azureProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := ap.getAzureConfigK8s()
	if err != nil {
		logrus.Errorf("Error fetching azure config: %v", err)
		return false, err
	}
	allowed, err := ap.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func UpdateGroupCacheSize(size string) {
	i, err := strconv.Atoi(size)
	if err != nil {
		logrus.Errorf("error parsing azure-group-cache-size, skipping update %v", err)
		return
	}
	if i < 0 {
		logrus.Error("azure-group-cache-size must be >= 0, skipping update")
		return
	}
	groupCache.Resize(i)
}
