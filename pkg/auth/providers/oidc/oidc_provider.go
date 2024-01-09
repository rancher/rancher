package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name      = "oidc"
	UserType  = "user"
	GroupType = "group"
)

type OpenIDCProvider struct {
	Name        string
	Type        string
	CTX         context.Context
	AuthConfigs v3.AuthConfigInterface
	Secrets     corev1.SecretInterface
	UserMGR     user.Manager
	TokenMGR    *tokens.Manager
}

type ClaimInfo struct {
	Subject           string   `json:"sub"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Groups            []string `json:"groups"`
	FullGroupPath     []string `json:"full_group_path"`
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	return &OpenIDCProvider{
		Name:        Name,
		Type:        client.OIDCConfigType,
		CTX:         ctx,
		AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
		Secrets:     mgmtCtx.Core.Secrets(""),
		UserMGR:     userMGR,
		TokenMGR:    tokenMGR,
	}
}

func (o *OpenIDCProvider) GetName() string {
	return Name
}

func (o *OpenIDCProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = o.ActionHandler
	schema.Formatter = o.Formatter
}

func (o *OpenIDCProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v32.OIDCLogin)
	if !ok {
		return v3.Principal{}, nil, "", fmt.Errorf("unexpected input type")
	}
	userPrincipal, groupPrincipals, providerToken, _, err := o.LoginUser(ctx, login, nil)
	return userPrincipal, groupPrincipals, providerToken, err
}

func (o *OpenIDCProvider) LoginUser(ctx context.Context, oauthLoginInfo *v32.OIDCLogin, config *v32.OIDCConfig) (v3.Principal, []v3.Principal, string, ClaimInfo, error) {
	var userPrincipal v3.Principal
	var groupPrincipals []v3.Principal
	var userClaimInfo ClaimInfo
	var err error

	if config == nil {
		config, err = o.GetOIDCConfig()
		if err != nil {
			return userPrincipal, nil, "", userClaimInfo, err
		}
	}
	userInfo, oauth2Token, err := o.getUserInfo(&ctx, config, oauthLoginInfo.Code, &userClaimInfo, "")
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	userPrincipal = o.userToPrincipal(userInfo, userClaimInfo)
	userPrincipal.Me = true
	groupPrincipals = o.getGroupsFromClaimInfo(userClaimInfo)

	logrus.Debugf("[generic oidc] loginuser: checking user's access to rancher")
	allowed, err := o.UserMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	if !allowed {
		return userPrincipal, groupPrincipals, "", userClaimInfo, httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}
	// save entire oauthToken because it contains refresh_token and token expiry time
	// will use with oauth2.Client and with TokenSource to ensure auto refresh of tokens occurs for api calls
	oauthToken, err := json.Marshal(oauth2Token)
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	return userPrincipal, groupPrincipals, string(oauthToken), userClaimInfo, nil
}

func (o *OpenIDCProvider) SearchPrincipals(searchValue, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal

	if principalType == "" {
		principalType = UserType
	}

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: o.Name + "_" + principalType + "://" + searchValue},
		DisplayName:   searchValue,
		LoginName:     searchValue,
		PrincipalType: principalType,
		Provider:      o.Name,
	}

	principals = append(principals, p)
	return principals, nil
}

func (o *OpenIDCProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	var p v3.Principal

	// parsing id to get the external id and type. Example oidc_<user|group>://<user sub | group name>
	var externalID string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return p, errors.Errorf("invalid id %v", principalID)
	}
	externalID = strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return p, errors.Errorf("invalid id %v", principalID)
	}

	principalType := parts[1]
	if externalID == "" && principalType == "" {
		return p, fmt.Errorf("invalid id %v", principalID)
	}
	if principalType != UserType && principalType != GroupType {
		return p, fmt.Errorf("invalid principal type")
	}
	if principalID == UserType {
		p = v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: principalType + "://" + externalID},
			DisplayName:   externalID,
			LoginName:     externalID,
			PrincipalType: UserType,
			Provider:      o.Name,
		}
	} else {
		p = o.groupToPrincipal(externalID)
	}
	p = o.toPrincipalFromToken(principalType, p, &token)
	return p, nil
}

func (o *OpenIDCProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.OIDCProviderFieldRedirectURL] = o.getRedirectURL(authConfig)
	return p, nil
}

func (o *OpenIDCProvider) getRedirectURL(config map[string]interface{}) string {
	return fmt.Sprintf(
		"%s?client_id=%s&response_type=code&redirect_uri=%s",
		config["authEndpoint"],
		config["clientId"],
		config["rancherUrl"],
	)
}

func (o *OpenIDCProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	var groupPrincipals []v3.Principal
	var claimInfo ClaimInfo

	config, err := o.GetOIDCConfig()
	if err != nil {
		logrus.Errorf("[generic oidc] refetchGroupPrincipals: error fetching OIDCConfig: %v", err)
		return groupPrincipals, err
	}
	// need to get the user information so that the refreshed token can be saved using the username / userID
	user, err := o.UserMGR.GetUserByPrincipalID(principalID)
	if err != nil {
		logrus.Errorf("[generic oidc] refetchGroupPrincipals: error getting user by principalID: %v", err)
		return groupPrincipals, err
	}
	//do not need userInfo or oauth2Token since we are only processing groups
	_, _, err = o.getUserInfo(&o.CTX, config, secret, &claimInfo, user.Name)
	if err != nil {
		return groupPrincipals, err
	}
	return o.getGroupsFromClaimInfo(claimInfo), nil
}

func (o *OpenIDCProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := o.GetOIDCConfig()
	if err != nil {
		logrus.Errorf("[generic oidc] canAccessWithGroupProviders: error fetching OIDCConfig: %v", err)
		return false, err
	}
	allowed, err := o.UserMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (o *OpenIDCProvider) userToPrincipal(userInfo *oidc.UserInfo, claimInfo ClaimInfo) v3.Principal {
	displayName := claimInfo.Name
	if displayName == "" {
		displayName = userInfo.Email
	}
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: o.Name + "_" + UserType + "://" + userInfo.Subject},
		DisplayName:   displayName,
		LoginName:     userInfo.Email,
		Provider:      o.Name,
		PrincipalType: UserType,
		Me:            false,
	}
	return p
}

func (o *OpenIDCProvider) groupToPrincipal(groupName string) v3.Principal {
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: o.Name + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		Provider:      o.Name,
		PrincipalType: GroupType,
		Me:            false,
	}
	return p
}

func (o *OpenIDCProvider) toPrincipalFromToken(principalType string, princ v3.Principal, token *v3.Token) v3.Principal {
	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = o.IsThisUserMe(token.UserPrincipal, princ)
			if princ.Me {
				princ.LoginName = token.UserPrincipal.LoginName
				princ.DisplayName = token.UserPrincipal.DisplayName
			}
		}
	} else {
		princ.PrincipalType = GroupType
		if token != nil {
			princ.MemberOf = o.TokenMGR.IsMemberOf(*token, princ)
		}
	}
	return princ
}

func (o *OpenIDCProvider) saveOIDCConfig(config *v32.OIDCConfig) error {
	storedOidcConfig, err := o.GetOIDCConfig()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = o.Type
	config.ObjectMeta = storedOidcConfig.ObjectMeta

	if config.PrivateKey != "" {
		privateKeyField := strings.ToLower(client.OIDCConfigFieldPrivateKey)
		if err = common.CreateOrUpdateSecrets(o.Secrets, config.PrivateKey, privateKeyField, strings.ToLower(config.Type)); err != nil {
			return err
		}
		config.PrivateKey = common.GetFullSecretName(config.Type, privateKeyField)
	}

	secretField := strings.ToLower(client.OIDCConfigFieldClientSecret)
	if err := common.CreateOrUpdateSecrets(o.Secrets, convert.ToString(config.ClientSecret), secretField, strings.ToLower(config.Type)); err != nil {
		return err
	}
	config.ClientSecret = common.GetFullSecretName(config.Type, secretField)

	logrus.Debugf("[generic oidc] saveOIDCConfig: updating config")
	_, err = o.AuthConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	return err
}

func (o *OpenIDCProvider) GetOIDCConfig() (*v32.OIDCConfig, error) {
	authConfigObj, err := o.AuthConfigs.ObjectClient().UnstructuredClient().Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, cannot read k8s Unstructured data")
	}
	storedOidcConfigMap := u.UnstructuredContent()

	storedOidcConfig := &v32.OIDCConfig{}
	err = common.Decode(storedOidcConfigMap, storedOidcConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode OidcConfig: %w", err)
	}

	if storedOidcConfig.PrivateKey != "" {
		value, err := common.ReadFromSecret(o.Secrets, storedOidcConfig.PrivateKey, strings.ToLower(client.OIDCConfigFieldPrivateKey))
		if err != nil {
			return nil, err
		}
		storedOidcConfig.PrivateKey = value
	}
	if storedOidcConfig.ClientSecret != "" {
		data, err := common.ReadFromSecretData(o.Secrets, storedOidcConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for _, v := range data {
			storedOidcConfig.ClientSecret = string(v)
		}
	}

	return storedOidcConfig, nil
}

func (o *OpenIDCProvider) IsThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (o *OpenIDCProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[common.UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[common.UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}

func (o *OpenIDCProvider) getUserInfo(ctx *context.Context, config *v32.OIDCConfig, authCode string, claimInfo *ClaimInfo, userName string) (*oidc.UserInfo, *oauth2.Token, error) {
	var userInfo *oidc.UserInfo
	var oauth2Token *oauth2.Token
	var err error

	updatedContext, err := AddCertKeyToContext(*ctx, config.Certificate, config.PrivateKey)
	if err != nil {
		return userInfo, oauth2Token, err
	}

	provider, err := oidc.NewProvider(updatedContext, config.Issuer)
	if err != nil {
		return userInfo, oauth2Token, err
	}
	oauthConfig := ConfigToOauthConfig(provider.Endpoint(), config)
	var verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	if err := json.Unmarshal([]byte(authCode), &oauth2Token); err != nil {
		oauth2Token, err = oauthConfig.Exchange(updatedContext, authCode, oauth2.SetAuthURLParam("scope", strings.Join(oauthConfig.Scopes, " ")))
		if err != nil {
			return userInfo, oauth2Token, err
		}
		_, err = verifier.Verify(updatedContext, oauth2Token.AccessToken)
		if err != nil {
			return userInfo, oauth2Token, err
		}
	}
	// Valid will return false if access token is expired
	if !oauth2Token.Valid() {
		// since token is not valid, the TokenSource func will attempt to refresh the access token
		// if the refresh token has not expired
		logrus.Debugf("[generic oidc] getUserInfo: attempting to refresh access token")
	}
	reusedToken, err := oauth2.ReuseTokenSource(oauth2Token, oauthConfig.TokenSource(updatedContext, oauth2Token)).Token()
	if err != nil {
		return userInfo, oauth2Token, err
	}
	if !reflect.DeepEqual(oauth2Token, reusedToken) {
		o.UpdateToken(reusedToken, userName)
	}
	logrus.Debugf("[generic oidc] getUserInfo: getting user info")
	userInfo, err = provider.UserInfo(updatedContext, oauthConfig.TokenSource(updatedContext, reusedToken))
	if err != nil {
		return userInfo, oauth2Token, err
	}
	if err := userInfo.Claims(&claimInfo); err != nil {
		return userInfo, oauth2Token, err
	}

	return userInfo, oauth2Token, nil
}

func ConfigToOauthConfig(endpoint oauth2.Endpoint, config *v32.OIDCConfig) oauth2.Config {
	var finalScopes []string
	hasOIDCScope := strings.Contains(config.Scopes, oidc.ScopeOpenID)
	// scopes must be space separated in string when passed into the api
	configScopes := strings.Split(config.Scopes, " ")
	if !hasOIDCScope {
		configScopes = append(configScopes, oidc.ScopeOpenID)
	}
	for _, scope := range configScopes {
		if scope != "" {
			finalScopes = append(finalScopes, scope)
		}
	}
	return oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  config.RancherURL,
		Scopes:       finalScopes,
	}
}

func (o *OpenIDCProvider) getGroupsFromClaimInfo(claimInfo ClaimInfo) []v3.Principal {
	var groupPrincipals []v3.Principal

	if claimInfo.FullGroupPath != nil {
		for _, groupPath := range claimInfo.FullGroupPath {
			groupsFromPath := strings.Split(groupPath, "/")
			for _, group := range groupsFromPath {
				if group != "" {
					groupPrincipal := o.groupToPrincipal(group)
					groupPrincipal.MemberOf = true
					groupPrincipals = append(groupPrincipals, groupPrincipal)
				}
			}
		}
	} else {
		for _, group := range claimInfo.Groups {
			groupPrincipal := o.groupToPrincipal(group)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}
	return groupPrincipals
}

func (o *OpenIDCProvider) UpdateToken(refreshedToken *oauth2.Token, userID string) error {
	var err error
	logrus.Debugf("[generic oidc] UpdateToken: access token has been refreshed")
	marshalledToken, err := json.Marshal(refreshedToken)
	if err != nil {
		return err
	}
	logrus.Debugf("[generic oidc] UpdateToken: saving refreshed access token")
	o.TokenMGR.UpdateSecret(userID, o.Name, string(marshalledToken))
	return err
}

// IsDisabledProvider checks if the OIDC auth provider is currently disabled in Rancher.
func (o *OpenIDCProvider) IsDisabledProvider() (bool, error) {
	oidcConfig, err := o.GetOIDCConfig()
	if err != nil {
		return false, err
	}
	return !oidcConfig.Enabled, nil
}
