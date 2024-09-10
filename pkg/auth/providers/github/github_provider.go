package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	util2 "github.com/rancher/rancher/pkg/auth/util"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name = "github"
)

type ghProvider struct {
	ctx          context.Context
	authConfigs  v3.AuthConfigInterface
	secrets      corev1.SecretInterface
	githubClient *GClient
	userMGR      user.Manager
	tokenMGR     *tokens.Manager
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	githubClient := &GClient{
		httpClient: &http.Client{},
	}

	return &ghProvider{
		ctx:          ctx,
		authConfigs:  mgmtCtx.Management.AuthConfigs(""),
		secrets:      mgmtCtx.Core.Secrets(""),
		githubClient: githubClient,
		userMGR:      userMGR,
		tokenMGR:     tokenMGR,
	}
}

func (g *ghProvider) GetName() string {
	return Name
}

func (g *ghProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = g.actionHandler
	schema.Formatter = g.formatter
}

func (g *ghProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.GithubProviderFieldRedirectURL] = formGithubRedirectURLFromMap(authConfig)
	return p, nil
}

func (g *ghProvider) getGithubConfigCR() (*v32.GithubConfig, error) {
	authConfigObj, err := g.authConfigs.ObjectClient().UnstructuredClient().Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, error: %v", err)
	}
	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, cannot read k8s Unstructured data")
	}
	storedGithubConfigMap := u.UnstructuredContent()

	storedGithubConfig := &v32.GithubConfig{}
	err = common.Decode(storedGithubConfigMap, storedGithubConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode Github Config: %w", err)
	}

	if storedGithubConfig.ClientSecret != "" {
		data, err := common.ReadFromSecretData(g.secrets, storedGithubConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for k, v := range data {
			if strings.EqualFold(k, client.GithubConfigFieldClientSecret) {
				storedGithubConfig.ClientSecret = string(v)
			} else {
				if storedGithubConfig.AdditionalClientIDs == nil {
					storedGithubConfig.AdditionalClientIDs = map[string]string{}
				}
				storedGithubConfig.AdditionalClientIDs[k] = strings.TrimSpace(string(v))
			}
		}
	}

	return storedGithubConfig, nil
}

func (g *ghProvider) saveGithubConfig(config *v32.GithubConfig) error {
	storedGithubConfig, err := g.getGithubConfigCR()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.GithubConfigType
	config.ObjectMeta = storedGithubConfig.ObjectMeta

	secretInfo := convert.ToString(config.ClientSecret)
	field := strings.ToLower(client.GithubConfigFieldClientSecret)
	if err := common.CreateOrUpdateSecrets(g.secrets, secretInfo, field, strings.ToLower(config.Type)); err != nil {
		return err
	}

	config.ClientSecret = common.GetFullSecretName(config.Type, field)

	_, err = g.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (g *ghProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v32.GithubLogin)
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

func choseClientID(host string, config *v32.GithubConfig) *v32.GithubConfig {
	if host == "" {
		return config
	}

	clientID := config.HostnameToClientID[host]
	secretID := config.AdditionalClientIDs[clientID]
	if secretID == "" {
		return config
	}

	copy := *config
	copy.ClientID = clientID
	copy.ClientSecret = secretID

	return &copy
}

func (g *ghProvider) LoginUser(host string, githubCredential *v32.GithubLogin, config *v32.GithubConfig, test bool) (v3.Principal, []v3.Principal, string, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var err error

	if config == nil {
		config, err = g.getGithubConfigCR()
		if err != nil {
			return v3.Principal{}, nil, "", err
		}
	}

	config = choseClientID(host, config)
	securityCode := githubCredential.Code

	accessToken, err := g.githubClient.getAccessToken(securityCode, config)
	if err != nil {
		logrus.Infof("Error generating accessToken from github %v", err)
		return v3.Principal{}, nil, "", err
	}

	user, err := g.githubClient.getUser(accessToken, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	userPrincipal = g.toPrincipal(userType, user, nil)
	userPrincipal.Me = true

	orgAccts, err := g.githubClient.getOrgs(accessToken, config)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	for _, orgAcct := range orgAccts {
		groupPrincipal := g.toPrincipal(orgType, orgAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts, err := g.githubClient.getTeams(accessToken, config)
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

	allowed, err := g.userMGR.CheckAccess(config.AccessMode, testAllowedPrincipals, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	if !allowed {
		return v3.Principal{}, nil, "", httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	return userPrincipal, groupPrincipals, accessToken, nil
}

func (g *ghProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var err error
	var config *v32.GithubConfig

	if config == nil {
		config, err = g.getGithubConfigCR()
		if err != nil {
			return nil, err
		}
	}

	orgAccts, err := g.githubClient.getOrgs(secret, config)
	if err != nil {
		return nil, err
	}
	for _, orgAcct := range orgAccts {
		groupPrincipal := g.toPrincipal(orgType, orgAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts, err := g.githubClient.getTeams(secret, config)
	if err != nil {
		return nil, err
	}
	for _, teamAcct := range teamAccts {
		groupPrincipal := g.toPrincipal(teamType, teamAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	return groupPrincipals, nil
}

func (g *ghProvider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, err := g.getGithubConfigCR()
	if err != nil {
		return principals, err
	}

	accessToken, err := g.tokenMGR.GetSecret(token.UserID, token.AuthProvider, []*v3.Token{&token})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		accessToken = token.ProviderInfo["access_token"]
	}

	accts, err := g.githubClient.searchUsers(searchKey, principalType, accessToken, config)
	if err != nil {
		logrus.Errorf("problem searching github: %v", err)
	}

	for _, acct := range accts {
		pType := strings.ToLower(acct.Type)
		if pType == "organization" {
			pType = orgType
		}
		p := g.toPrincipal(pType, acct, &token)
		principals = append(principals, p)
	}

	return principals, nil
}

const (
	userType = "user"
	teamType = "team"
	orgType  = "org"
)

func (g *ghProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	config, err := g.getGithubConfigCR()
	if err != nil {
		return v3.Principal{}, err
	}

	accessToken, err := g.tokenMGR.GetSecret(token.UserID, token.AuthProvider, []*v3.Token{&token})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return v3.Principal{}, err
		}
		accessToken = token.ProviderInfo["access_token"]
	}
	// parsing id to get the external id and type. id looks like github_[user|org|team]://12345
	var externalID string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}
	externalID = strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}

	principalType := parts[1]
	var acct Account
	switch principalType {
	case userType:
		fallthrough
	case orgType:
		acct, err = g.githubClient.getUserOrgByID(externalID, accessToken, config)
		if err != nil {
			return v3.Principal{}, err
		}
	case teamType:
		acct, err = g.githubClient.getTeamByID(externalID, accessToken, config)
		if err != nil {
			return v3.Principal{}, err
		}
	default:
		return v3.Principal{}, fmt.Errorf("Cannot get the github account due to invalid externalIDType %v", principalType)
	}

	princ := g.toPrincipal(principalType, acct, &token)
	return princ, nil
}

func (g *ghProvider) toPrincipal(principalType string, acct Account, token *v3.Token) v3.Principal {
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
			princ.Me = g.isThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = g.tokenMGR.IsMemberOf(*token, princ)
		}
	}

	return princ
}

func (g *ghProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {

	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (g *ghProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := g.getGithubConfigCR()
	if err != nil {
		logrus.Errorf("Error fetching github config: %v", err)
		return false, err
	}
	allowed, err := g.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (g *ghProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[common.UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[common.UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}

// IsDisabledProvider checks if the GitHub auth provider is currently disabled in Rancher.
func (g *ghProvider) IsDisabledProvider() (bool, error) {
	ghConfig, err := g.getGithubConfigCR()
	if err != nil {
		return false, err
	}
	return !ghConfig.Enabled, nil
}
