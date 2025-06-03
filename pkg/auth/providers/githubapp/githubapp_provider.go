package githubapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	cattlev3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// Name is used to reference the Provider.
	Name = "githubapp"

	userType = "user"
	teamType = "team"
	orgType  = "org"
)

type githubClient interface {
	getAccessToken(ctx context.Context, code string, config *cattlev3.GithubAppConfig) (string, error)
	getUser(ctx context.Context, githubAccessToken string, config *cattlev3.GithubAppConfig) (Account, error)
	getOrgsForUser(ctx context.Context, username string, config *cattlev3.GithubAppConfig) ([]Account, error)
	getTeamsForUser(ctx context.Context, username string, config *cattlev3.GithubAppConfig) ([]Account, error)
	searchUsers(ctx context.Context, searchTerm, searchType string, config *cattlev3.GithubAppConfig) ([]Account, error)
	searchTeams(ctx context.Context, searchTerm string, config *cattlev3.GithubAppConfig) ([]Account, error)
	getUserOrgByID(ctx context.Context, id int, config *cattlev3.GithubAppConfig) (Account, error)
	getTeamByID(ctx context.Context, id int, config *cattlev3.GithubAppConfig) (Account, error)
}

type tokensManager interface {
	GetSecret(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error)
	IsMemberOf(token accessor.TokenAccessor, group v3.Principal) bool
	CreateTokenAndSetCookie(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error
	UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []v3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error
}

type userManager interface {
	CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error)
	SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error)
}

type ghAppProvider struct {
	ctx           context.Context
	authConfigs   v3.AuthConfigInterface
	secrets       wcorev1.SecretController
	getConfig     func() (*cattlev3.GithubAppConfig, error)
	githubClient  githubClient
	userManager   userManager
	tokenMGR      tokensManager
	appDataLoader func(ctx context.Context, appID int64, privateKey []byte, installationID int64, endpoint string) (*gitHubAppData, error)
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userManager user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	provider := &ghAppProvider{
		ctx:          ctx,
		authConfigs:  mgmtCtx.Management.AuthConfigs(""),
		secrets:      mgmtCtx.Wrangler.Core.Secret(),
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		userManager:  userManager,
		tokenMGR:     tokenMGR,
	}
	provider.getConfig = provider.getGithubAppConfigCR

	return provider
}

func (g *ghAppProvider) LogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	return nil
}

func (g *ghAppProvider) Logout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	return nil
}

func (g *ghAppProvider) GetName() string {
	return Name
}

func (g *ghAppProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = g.actionHandler
	schema.Formatter = g.formatter
}

func (g *ghAppProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.GithubProviderFieldRedirectURL] = formGithubRedirectURLFromMap(authConfig)
	return p, nil
}

func (g *ghAppProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*cattlev3.GithubLogin)
	if !ok {
		return v3.Principal{}, nil, "", errors.New("unexpected input type")
	}
	host := ""
	req, ok := ctx.Value(util2.RequestKey).(*http.Request)
	if ok {
		host = util2.GetHost(req)
	}

	return g.LoginUser(host, login, nil, false)
}

func (g *ghAppProvider) LoginUser(host string, githubCredential *cattlev3.GithubLogin, config *cattlev3.GithubAppConfig, test bool) (v3.Principal, []v3.Principal, string, error) {
	var err error

	if config == nil {
		config, err = g.getConfig()
		if err != nil {
			return v3.Principal{}, nil, "", err
		}
	}

	config = chooseClientID(host, config)
	securityCode := githubCredential.Code

	logrus.Info("ghAppProvider.LoginUser")

	ctx := context.Background()
	accessToken, err := g.githubClient.getAccessToken(ctx, securityCode, config)
	if err != nil {
		logrus.Infof("Error generating accessToken from github %v", err)
		return v3.Principal{}, nil, "", err
	}

	user, err := g.githubClient.getUser(ctx, accessToken, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	userPrincipal := g.toPrincipal(userType, user, nil)
	userPrincipal.Me = true

	var groupPrincipals []v3.Principal
	orgAccts, err := g.githubClient.getOrgsForUser(ctx, user.Login, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	for _, orgAcct := range orgAccts {
		groupPrincipal := g.toPrincipal(orgType, orgAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts, err := g.githubClient.getTeamsForUser(ctx, user.Login, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	for _, teamAcct := range teamAccts {
		groupPrincipal := g.toPrincipal(teamType, teamAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	testAllowedPrincipals := config.AllowedPrincipalIDs
	if test && config.AccessMode == "restricted" {
		testAllowedPrincipals = append(testAllowedPrincipals, userPrincipal.Name)
	}

	allowed, err := g.userManager.CheckAccess(config.AccessMode, testAllowedPrincipals, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	if !allowed {
		return v3.Principal{}, nil, "", httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	// Does not return the Token which is a user-OAuth token.
	return userPrincipal, groupPrincipals, "", nil
}

// The Secret parameter is unused in this provider.
func (g *ghAppProvider) RefetchGroupPrincipals(principalID string, _ string) ([]v3.Principal, error) {
	config, err := g.getConfig()
	if err != nil {
		return nil, err
	}

	return g.getGroupPrincipals(principalID, config)
}

func (g *ghAppProvider) getGroupPrincipals(principalID string, config *cattlev3.GithubAppConfig) ([]v3.Principal, error) {
	// Should this check for a user Principal?
	_, id, err := parsePrincipalID(principalID)
	if err != nil {
		return nil, err
	}

	var groupPrincipals []v3.Principal

	data, err := getAppDataWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	member := data.findMemberByID(id)
	if member == nil {
		return groupPrincipals, nil
	}

	orgAccts := data.listOrgsForUser(member.Login)
	for _, orgAcct := range orgAccts {
		groupPrincipal := g.toPrincipal(orgType, orgAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts := data.listTeamsForUser(member.Login)
	for _, teamAcct := range teamAccts {
		groupPrincipal := g.toPrincipal(teamType, teamAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	return groupPrincipals, nil
}

// SearchPrincipals queries the app data matching on strings.
//
// The principalType can be user or group.
func (g *ghAppProvider) SearchPrincipals(searchKey, principalType string, token accessor.TokenAccessor) ([]v3.Principal, error) {
	config, err := g.getConfig()
	if err != nil {
		return nil, err
	}

	var principals []v3.Principal
	ctx := context.Background()
	// TODO: Should this search within the orgs that a user is a member of?
	// It was discussed that we'd search with the credentials of the app.
	users, err := g.githubClient.searchUsers(ctx, searchKey, principalType, config)
	if err != nil {
		return nil, err
	}

	for _, acct := range users {
		pType := strings.ToLower(acct.Type)
		if pType == "organization" {
			pType = orgType
		}
		principals = append(principals, g.toPrincipal(pType, acct, token))
	}

	if principalType == "" || principalType == "group" {
		teamAccts, err := g.githubClient.searchTeams(ctx, searchKey, config)
		if err != nil {
			return nil, err
		}
		for _, acct := range teamAccts {
			p := g.toPrincipal(teamType, acct, token)
			principals = append(principals, p)
		}
	}

	return principals, nil
}

func (g *ghAppProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error) {
	config, err := g.getConfig()
	if err != nil {
		return v3.Principal{}, err
	}
	ctx := context.Background()

	principalType, externalID, err := parsePrincipalID(principalID)
	switch principalType {
	case userType, orgType:
		acct, err := g.githubClient.getUserOrgByID(ctx, externalID, config)
		if err != nil {
			return v3.Principal{}, err
		}
		return g.toPrincipal(principalType, acct, token), nil
	case teamType:
		acct, err := g.githubClient.getTeamByID(ctx, externalID, config)
		if err != nil {
			return v3.Principal{}, err
		}
		return g.toPrincipal(principalType, acct, token), nil
	default:
		return v3.Principal{}, fmt.Errorf("cannot get the github account due to invalid externalID Type %v", principalType)
	}
}

func (g *ghAppProvider) toPrincipal(principalType string, acct Account, token accessor.TokenAccessor) v3.Principal {
	displayName := acct.Name
	if displayName == "" {
		displayName = acct.Login
	}

	princ := v3.Principal{
		ObjectMeta:     metav1.ObjectMeta{Name: Name + "_" + principalType + "://" + strconv.Itoa(acct.ID)},
		DisplayName:    displayName,
		LoginName:      acct.Login,
		Provider:       Name,
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
			princ.MemberOf = g.tokenMGR.IsMemberOf(token, princ)
		}
	}

	return princ
}

func (g *ghAppProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := g.getConfig()
	if err != nil {
		logrus.Errorf("Error fetching github config: %v", err)
		return false, err
	}
	allowed, err := g.userManager.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (g *ghAppProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider checks if the GitHub auth provider is currently disabled in Rancher.
func (g *ghAppProvider) IsDisabledProvider() (bool, error) {
	ghConfig, err := g.getConfig()
	if err != nil {
		return false, err
	}
	return !ghConfig.Enabled, nil
}

func (g *ghAppProvider) getGithubAppConfigCR() (*cattlev3.GithubAppConfig, error) {
	authConfigObj, err := g.authConfigs.ObjectClient().UnstructuredClient().Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting GithubAppConfig, error: %v", err)
	}
	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("parsing GithubAppConfig, cannot read k8s Unstructured data")
	}
	storedGithubAppConfigMap := u.UnstructuredContent()

	storedGithubAppConfig := &cattlev3.GithubAppConfig{}
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

	return storedGithubAppConfig, nil
}

func (g *ghAppProvider) saveGithubAppConfig(config *cattlev3.GithubAppConfig) error {
	storedGithubAppConfig, err := g.getGithubAppConfigCR()
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

	_, err = g.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func getAppDataWithConfig(ctx context.Context, config *cattlev3.GithubAppConfig) (*gitHubAppData, error) {
	appID, err := strconv.ParseInt(config.AppID, 10, 64)
	// TODO: test
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

func chooseClientID(host string, sourceConfig *cattlev3.GithubAppConfig) *cattlev3.GithubAppConfig {
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

func parsePrincipalID(s string) (kind string, id int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid id %s", s)
	}
	externalID := strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid id %s", s)
	}

	principalType := parts[1]

	principalID, err := strconv.Atoi(externalID)
	if err != nil {
		return "", 0, fmt.Errorf("invalid id %s", s)
	}

	return principalType, principalID, nil
}
