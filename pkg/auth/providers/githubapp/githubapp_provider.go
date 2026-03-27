package githubapp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	util2 "github.com/rancher/rancher/pkg/auth/util"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// ProviderName is used to reference the Provider.
	ProviderName = "githubapp"

	userType = "user"
	teamType = "team"
	orgType  = "org"
)

type githubClient interface {
	getAccessToken(ctx context.Context, code string, config *apiv3.GithubAppConfig) (string, error)
	getUser(ctx context.Context, githubAccessToken string, config *apiv3.GithubAppConfig) (common.GitHubAccount, error)
	getOrgsForUser(ctx context.Context, username string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error)
	getTeamsForUser(ctx context.Context, username string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error)
	searchUsers(ctx context.Context, searchTerm, searchType string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error)
	searchTeams(ctx context.Context, searchTerm string, config *apiv3.GithubAppConfig) ([]common.GitHubAccount, error)
	getUserOrgByID(ctx context.Context, id int, config *apiv3.GithubAppConfig) (common.GitHubAccount, error)
	getTeamByID(ctx context.Context, id int, config *apiv3.GithubAppConfig) (common.GitHubAccount, error)
}

type tokensManager interface {
	GetSecret(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error)
	CreateTokenAndSetCookie(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error
}

type userManager interface {
	CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []apiv3.Principal) (bool, error)
	IsMemberOf(token accessor.TokenAccessor, group apiv3.Principal) bool
	SetPrincipalOnCurrentUser(r *http.Request, principal apiv3.Principal) (*apiv3.User, error)
	UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []apiv3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error
}

type Provider struct {
	authConfigs  v3.AuthConfigInterface
	secrets      wcorev1.SecretController
	getConfig    func(string) (*apiv3.GithubAppConfig, error)
	githubClient githubClient
	userManager  userManager
	tokenMGR     tokensManager
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userManager user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	provider := &Provider{
		authConfigs:  mgmtCtx.Management.AuthConfigs(""),
		secrets:      mgmtCtx.Wrangler.Core.Secret(),
		githubClient: &githubAppClient{httpClient: common.NewHTTPClientWithTimeouts()},
		userManager:  userManager,
		tokenMGR:     tokenMGR,
	}
	provider.getConfig = provider.getGithubAppConfigCR

	return provider
}

// LogoutAll is not implemented in the GitHubApp provider because we don't
// currently terminate the OAuth session.
//
// We might choose to do this in future:
// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/token-expiration-and-revocation#token-revoked-by-the-oauth-app
func (g *Provider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	return nil
}

// Logout is not implemented in the GitHubApp provider because we don't
// differentiate between Logout and LogoutAll in this provider.
func (g *Provider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	return nil
}

func (g *Provider) GetName() string {
	return ProviderName
}

func (g *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = g.actionHandler
	schema.Formatter = g.formatter
}

func (g *Provider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.GithubProviderFieldRedirectURL] = formGithubRedirectURLFromMap(authConfig)
	return p, nil
}

func (g *Provider) AuthenticateUser(_ http.ResponseWriter, req *http.Request, input any) (apiv3.Principal, []apiv3.Principal, string, error) {
	login, ok := input.(*apiv3.GithubLogin)
	if !ok {
		return apiv3.Principal{}, nil, "", errors.New("unexpected input type")
	}

	return g.LoginUser(util2.GetHost(req), login, nil, false)
}

func (g *Provider) LoginUser(host string, githubCredential *apiv3.GithubLogin, config *apiv3.GithubAppConfig, test bool) (apiv3.Principal, []apiv3.Principal, string, error) {
	var err error

	// TODO: Ensure that these are populated!
	configName := cmp.Or(githubCredential.ConfigName, ProviderName)
	if config == nil {
		logrus.Debugf("Loading GithubApp config %s", configName)
		config, err = g.getConfig(configName)
		if err != nil {
			return apiv3.Principal{}, nil, "", err
		}
	}

	config = chooseClientID(host, config)
	securityCode := githubCredential.Code

	ctx := context.Background()
	accessToken, err := g.githubClient.getAccessToken(ctx, securityCode, config)
	if err != nil {
		logrus.Infof("Error generating accessToken from github %v", err)
		return apiv3.Principal{}, nil, "", err
	}

	user, err := g.githubClient.getUser(ctx, accessToken, config)
	if err != nil {
		return apiv3.Principal{}, nil, "", err
	}
	userPrincipal := g.toPrincipal(userType, user, configName, nil)
	userPrincipal.Me = true

	var groupPrincipals []apiv3.Principal
	orgAccts, err := g.githubClient.getOrgsForUser(ctx, user.Login, config)
	if err != nil {
		return apiv3.Principal{}, nil, "", err
	}
	for _, orgAcct := range orgAccts {
		groupPrincipal := g.toPrincipal(orgType, orgAcct, configName, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts, err := g.githubClient.getTeamsForUser(ctx, user.Login, config)
	if err != nil {
		return apiv3.Principal{}, nil, "", err
	}
	for _, teamAcct := range teamAccts {
		groupPrincipal := g.toPrincipal(teamType, teamAcct, configName, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	testAllowedPrincipals := config.AllowedPrincipalIDs
	if test && config.AccessMode == "restricted" {
		testAllowedPrincipals = append(testAllowedPrincipals, userPrincipal.Name)
	}

	allowed, err := g.userManager.CheckAccess(config.AccessMode, testAllowedPrincipals, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return apiv3.Principal{}, nil, "", err
	}
	if !allowed {
		return apiv3.Principal{}, nil, "", apierror.NewAPIError(validation.Unauthorized, "unauthorized")
	}

	// Does not return the Token which is a user-OAuth token.
	return userPrincipal, groupPrincipals, "", nil
}

// The Secret parameter is unused in this provider.
func (g *Provider) RefetchGroupPrincipals(principalID string, _ string) ([]apiv3.Principal, error) {
	configName, _, _, err := common.SplitPrincipalID(principalID)
	if err != nil {
		return nil, err
	}

	config, err := g.getConfig(configName)
	if err != nil {
		return nil, err
	}

	return g.getGroupPrincipals(principalID, config)
}

func (g *Provider) UsesUserSecrets() bool      { return false }
func (g *Provider) CanRefreshPrincipals() bool { return true }

func (g *Provider) getGroupPrincipals(principalID string, config *apiv3.GithubAppConfig) ([]apiv3.Principal, error) {
	var groupPrincipals []apiv3.Principal
	data, err := getAppDataWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	_, _, externalID, err := common.SplitPrincipalID(principalID)
	if err != nil {
		return nil, err
	}

	memberID, err := strconv.Atoi(externalID)
	if err != nil {
		return nil, fmt.Errorf("invalid github app principal ID %s: %w", principalID, err)
	}

	member := data.findMemberByID(memberID)
	if member == nil {
		return groupPrincipals, nil
	}

	orgAccts := data.listOrgsForUser(member.Login)
	for _, orgAcct := range orgAccts {
		groupPrincipal := g.toPrincipal(orgType, orgAcct, config.Name, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts := data.listTeamsForUser(member.Login)
	for _, teamAcct := range teamAccts {
		groupPrincipal := g.toPrincipal(teamType, teamAcct, config.Name, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	return groupPrincipals, nil
}

// SearchPrincipals queries the app data matching on strings.
//
// The principalType can be user or group.
func (g *Provider) SearchPrincipals(searchKey, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	// Use the authenticated token's principal to get the config to search.
	// This will not search cross providers.
	configName, err := common.ConfigNameFromToken(token)
	if err != nil {
		return nil, err
	}
	config, err := g.getConfig(configName)
	if err != nil {
		return nil, err
	}

	var principals []apiv3.Principal
	ctx := context.Background()

	users, err := g.githubClient.searchUsers(ctx, searchKey, principalType, config)
	if err != nil {
		return nil, err
	}

	for _, acct := range users {
		pType := strings.ToLower(acct.Type)
		if pType == "organization" {
			pType = orgType
		}
		principals = append(principals, g.toPrincipal(pType, acct, configName, token))
	}

	if principalType == "" || principalType == "group" {
		teamAccts, err := g.githubClient.searchTeams(ctx, searchKey, config)
		if err != nil {
			return nil, err
		}
		for _, acct := range teamAccts {
			p := g.toPrincipal(teamType, acct, configName, token)
			principals = append(principals, p)
		}
	}

	return principals, nil
}

func (g *Provider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	// Use the authenticated token's principal to get the config to get the
	// principal from.
	// This will not work cross providers.
	configName, err := common.ConfigNameFromToken(token)
	if err != nil {
		return apiv3.Principal{}, err
	}
	config, err := g.getConfig(configName)
	if err != nil {
		return apiv3.Principal{}, err
	}

	_, principalType, externalID, err := common.SplitPrincipalID(principalID)
	if err != nil {
		return apiv3.Principal{}, err
	}

	memberID, err := strconv.Atoi(externalID)
	if err != nil {
		return apiv3.Principal{}, fmt.Errorf("invalid github app principal ID %s: %w", principalID, err)
	}

	ctx := context.Background()
	switch principalType {
	case userType, orgType:
		acct, err := g.githubClient.getUserOrgByID(ctx, memberID, config)
		if err != nil {
			return apiv3.Principal{}, err
		}
		return g.toPrincipal(principalType, acct, configName, token), nil
	case teamType:
		acct, err := g.githubClient.getTeamByID(ctx, memberID, config)
		if err != nil {
			return apiv3.Principal{}, err
		}
		return g.toPrincipal(principalType, acct, configName, token), nil
	default:
		return apiv3.Principal{}, fmt.Errorf("cannot get the github account due to invalid externalID Type %v", principalType)
	}
}

func (g *Provider) toPrincipal(principalType string, acct common.GitHubAccount, configName string, token accessor.TokenAccessor) apiv3.Principal {
	displayName := acct.Name
	if displayName == "" {
		displayName = acct.Login
	}

	princ := apiv3.Principal{
		ObjectMeta:     metav1.ObjectMeta{Name: configName + "_" + principalType + "://" + strconv.Itoa(acct.ID)},
		DisplayName:    displayName,
		LoginName:      acct.Login,
		Provider:       ProviderName,
		Me:             false,
		ProfilePicture: acct.AvatarURL,
	}

	if principalType == userType {
		princ.PrincipalType = "user"
		if token != nil {
			princ.Me = common.SamePrincipal(token.GetUserPrincipal(), princ)
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = g.userManager.IsMemberOf(token, princ)
		}
	}

	return princ
}

func (g *Provider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []apiv3.Principal) (bool, error) {
	configName, _, _, err := common.SplitPrincipalID(userPrincipalID)
	if err != nil {
		return false, err
	}
	config, err := g.getConfig(configName)
	if err != nil {
		return false, err
	}

	return g.userManager.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
}

func (g *Provider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider checks if the GitHub auth provider is currently disabled in Rancher.
func (g *Provider) IsDisabledProvider(configName string) (bool, error) {
	ghConfig, err := g.getConfig(configName)
	if err != nil {
		return false, err
	}
	return !ghConfig.Enabled, nil
}

func (g *Provider) getGithubAppConfigCR(configName string) (*apiv3.GithubAppConfig, error) {
	authConfigObj, err := g.authConfigs.ObjectClient().UnstructuredClient().Get(configName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting GithubAppConfig %s, error: %v", configName, err)
	}
	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("parsing GithubAppConfig, cannot read k8s Unstructured data")
	}
	storedGithubAppConfigMap := u.UnstructuredContent()

	storedGithubAppConfig := &apiv3.GithubAppConfig{}
	err = common.Decode(storedGithubAppConfigMap, storedGithubAppConfig)
	if err != nil {
		return nil, fmt.Errorf("decode GithubApp Config: %w", err)
	}

	if storedGithubAppConfig.ClientSecret != "" {
		data, err := common.ReadFromSecretData(g.secrets, storedGithubAppConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for k, v := range data {
			if strings.EqualFold(k, client.GithubAppConfigFieldClientSecret) {
				storedGithubAppConfig.ClientSecret = string(v)
			} else {
				if storedGithubAppConfig.AdditionalClientIDs == nil {
					storedGithubAppConfig.AdditionalClientIDs = map[string]string{}
				}
				storedGithubAppConfig.AdditionalClientIDs[k] = strings.TrimSpace(string(v))
			}
		}
	}

	if storedGithubAppConfig.PrivateKey != "" {
		data, err := common.ReadFromSecretData(g.secrets, storedGithubAppConfig.PrivateKey)
		if err != nil {
			return nil, err
		}
		for k, v := range data {
			if strings.EqualFold(k, client.GithubAppConfigFieldPrivateKey) {
				storedGithubAppConfig.PrivateKey = string(v)
			}
		}
	}

	return storedGithubAppConfig, nil
}

func (g *Provider) saveGithubAppConfig(config *apiv3.GithubAppConfig) error {
	storedGithubAppConfig, err := g.getGithubAppConfigCR(config.GetName())
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.GithubAppConfigType
	config.ObjectMeta = storedGithubAppConfig.ObjectMeta

	secretInfo := convert.ToString(config.ClientSecret)
	field := strings.ToLower(client.GithubAppConfigFieldClientSecret)
	name, err := common.CreateOrUpdateSecrets(g.secrets, secretInfo, field, strings.ToLower(config.Type))
	if err != nil {
		return err
	}
	config.ClientSecret = name

	privateKeyInfo := convert.ToString(config.PrivateKey)
	privateKeyField := strings.ToLower(client.GithubAppConfigFieldPrivateKey)
	privateKeyName, err := common.CreateOrUpdateSecrets(g.secrets, privateKeyInfo, privateKeyField, strings.ToLower(config.Type))
	if err != nil {
		return err
	}

	config.PrivateKey = privateKeyName

	_, err = g.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}

	return nil
}

func getAppDataWithConfig(ctx context.Context, config *apiv3.GithubAppConfig) (*gitHubAppData, error) {
	appID, err := strconv.ParseInt(config.AppID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing GitHub App ID: %w", err)
	}

	var installationID int64
	if config.InstallationID != "" {
		i, err := strconv.ParseInt(config.InstallationID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing GitHub installation ID: %w", err)
		}
		installationID = i
	}

	return getDataForApp(ctx, appID, []byte(config.PrivateKey), installationID, getAPIURL("", config))
}

func chooseClientID(host string, sourceConfig *apiv3.GithubAppConfig) *apiv3.GithubAppConfig {
	if host == "" {
		return sourceConfig
	}

	clientID := sourceConfig.HostnameToClientID[host]
	secretID := sourceConfig.AdditionalClientIDs[clientID]
	if secretID == "" {
		return sourceConfig
	}

	config := *sourceConfig
	config.ClientID = clientID
	config.ClientSecret = secretID

	return &config
}
