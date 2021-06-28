package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/mitchellh/mapstructure"
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

type claimInfo struct {
	Subject           string   `json:"sub"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:preferred_username`
	GivenName         string   `json:given_name`
	FamilyName        string   `json:family_name`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Groups            []string `json:"groups"`
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
	return o.LoginUser(ctx, login, nil)
}

func (o *OpenIDCProvider) LoginUser(ctx context.Context, oauthLoginInfo *v32.OIDCLogin, config *v32.OIDCConfig) (v3.Principal, []v3.Principal, string, error) {
	var userPrincipal v3.Principal
	var groupPrincipals []v3.Principal
	var claimInfo claimInfo
	var err error

	if config == nil {
		config, err = o.GetOIDCConfig()
		if err != nil {
			return userPrincipal, nil, "", err
		}
	}
	userInfo, oauth2Token, err := o.getUserInfo(&ctx, config, oauthLoginInfo.Code, &claimInfo)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}

	userPrincipal = o.userToPrincipal(userInfo, claimInfo)
	userPrincipal.Me = true

	for _, group := range claimInfo.Groups {
		groupPrincipal := o.groupToPrincipal(group)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)

	}
	logrus.Debugf("[generic oidc] loginuser: Checking user's access to Rancher")
	allowed, err := o.UserMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}
	if !allowed {
		return userPrincipal, groupPrincipals, "", httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}
	// save entire oauthToken because it contains refresh_token and token expiry time
	// will use with oauth2.Client and with TokenSource to ensure auto refresh of tokens occurs for api calls
	oauthToken, err := json.Marshal(oauth2Token)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}
	return userPrincipal, groupPrincipals, string(oauthToken), nil
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
		return p, fmt.Errorf("[generic oidc]: invalid id %v", principalID)
	}
	if principalType != UserType && principalType != GroupType {
		return p, fmt.Errorf("[generic oidc]: invalid principal type")
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
	var claimInfo claimInfo

	config, err := o.GetOIDCConfig()
	if err != nil {
		logrus.Errorf("[generic oidc]: error fetching OIDCConfig: %v", err)
		return groupPrincipals, err
	}
	//do not need userInfo or oauth2Token since we are only processing groups
	_, _, err = o.getUserInfo(&o.CTX, config, secret, &claimInfo)
	if err != nil {
		return groupPrincipals, err
	}
	for _, group := range claimInfo.Groups {
		groupPrincipal := o.groupToPrincipal(group)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)

	}
	return groupPrincipals, nil
}

func (o *OpenIDCProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := o.GetOIDCConfig()
	if err != nil {
		logrus.Errorf("[generic oidc]: error fetching OIDCConfig: %v", err)
		return false, err
	}
	allowed, err := o.UserMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (o *OpenIDCProvider) userToPrincipal(userInfo *oidc.UserInfo, claimInfo claimInfo) v3.Principal {
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
		config.PrivateKey = common.GetName(config.Type, privateKeyField)
	}

	secretField := strings.ToLower(client.OIDCConfigFieldClientSecret)
	if err := common.CreateOrUpdateSecrets(o.Secrets, convert.ToString(config.ClientSecret), secretField, strings.ToLower(config.Type)); err != nil {
		return err
	}
	config.ClientSecret = common.GetName(config.Type, secretField)

	logrus.Debugf("[generic oidc] updating config")
	_, err = o.AuthConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	return err
}

func (o *OpenIDCProvider) GetOIDCConfig() (*v32.OIDCConfig, error) {
	authConfigObj, err := o.AuthConfigs.ObjectClient().UnstructuredClient().Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("[generic oidc]: failed to retrieve OIDCConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("[generic oidc]: failed to retrieve OIDCConfig, cannot read k8s Unstructured data")
	}
	storedOidcConfigMap := u.UnstructuredContent()

	storedOidcConfig := &v32.OIDCConfig{}
	mapstructure.Decode(storedOidcConfigMap, storedOidcConfig)

	metadataMap, ok := storedOidcConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("[generic oidc]: failed to retrieve OIDCConfig metadata, cannot read k8s Unstructured data")
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedOidcConfig.ObjectMeta = *objectMeta

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

func (o *OpenIDCProvider) GetUserExtraAttributes(token *v3.Token) map[string][]string {
	extras := make(map[string][]string)
	extras["principalid"] = []string{token.UserPrincipal.Name}
	extras["username"] = []string{token.UserPrincipal.LoginName}
	return extras
}

func (o *OpenIDCProvider) getUserInfo(ctx *context.Context, config *v32.OIDCConfig, authCode string, claimInfo *claimInfo) (*oidc.UserInfo, *oauth2.Token, error) {
	var userInfo *oidc.UserInfo
	var oauth2Token *oauth2.Token
	var err error

	err = o.AddCertKeyToContext(ctx, config.Certificate, config.PrivateKey)
	if err != nil {
		return userInfo, oauth2Token, err
	}

	provider, err := oidc.NewProvider(*ctx, config.Issuer)
	if err != nil {
		return userInfo, oauth2Token, err
	}
	oauthConfig := ConfigToOauthConfig(provider.Endpoint(), config)
	if err := json.Unmarshal([]byte(authCode), &oauth2Token); err != nil {
		oauth2Token, err = oauthConfig.Exchange(*ctx, authCode, oauth2.SetAuthURLParam("scope", strings.Join(oauthConfig.Scopes, " ")))
		if err != nil {
			return userInfo, oauth2Token, err
		}
	}

	rawToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		rawToken, ok = oauth2Token.Extra("access_token").(string)
		if !ok {
			return userInfo, oauth2Token, err
		}
	}
	var verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	// parse and verify the id token payload
	_, err = verifier.Verify(*ctx, rawToken)
	if err != nil {
		return userInfo, oauth2Token, err
	}
	userInfo, err = provider.UserInfo(*ctx, oauthConfig.TokenSource(*ctx, oauth2Token))
	if err != nil {
		return userInfo, oauth2Token, err
	}
	if err := userInfo.Claims(&claimInfo); err != nil {
		return userInfo, oauth2Token, err
	}
	return userInfo, oauth2Token, nil
}

func ConfigToOauthConfig(endpoint oauth2.Endpoint, config *v32.OIDCConfig) oauth2.Config {
	configScopes := strings.Split(config.Scopes, ",")
	allScopes := []string{oidc.ScopeOpenID}
	allScopes = append(allScopes, configScopes...)

	return oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  config.RancherURL,
		Scopes:       allScopes,
	}
}
