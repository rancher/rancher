package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
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

type tokenManager interface {
	IsMemberOf(token accessor.TokenAccessor, group v3.Principal) bool
	UpdateSecret(userID, provider, secret string) error
	UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []v3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error
	CreateTokenAndSetCookie(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error
	GetSecret(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error)
}

type OpenIDCProvider struct {
	Name        string
	Type        string
	CTX         context.Context
	AuthConfigs v3.AuthConfigInterface
	Secrets     wcorev1.SecretController
	UserMGR     user.Manager
	TokenMgr    tokenManager
	GetConfig   func() (*v32.OIDCConfig, error)
}

type ClaimInfo struct {
	Subject       string   `json:"sub"`
	Name          string   `json:"name"`
	Email         string   `json:"email"`
	Groups        []string `json:"groups"`
	FullGroupPath []string `json:"full_group_path"`

	// https://openid.net/specs/openid-connect-core-1_0.html#rfc.section.2
	ACR string `json:"acr"`

	Roles []string `json:"roles"`
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, TokenMgr *tokens.Manager) common.AuthProvider {
	p := &OpenIDCProvider{
		Name:        Name,
		Type:        client.OIDCConfigType,
		CTX:         ctx,
		AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
		Secrets:     mgmtCtx.Wrangler.Core.Secret(),
		UserMGR:     userMGR,
		TokenMgr:    TokenMgr,
	}

	p.GetConfig = p.GetOIDCConfig
	return p
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
		config, err = o.GetConfig()
		if err != nil {
			return userPrincipal, nil, "", userClaimInfo, err
		}
	}

	userInfo, oauth2Token, err := o.getUserInfoFromAuthCode(&ctx, config, oauthLoginInfo.Code, &userClaimInfo, "")
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	userPrincipal = o.userToPrincipal(userInfo, userClaimInfo)
	userPrincipal.Me = true
	groupPrincipals = o.getGroupsFromClaimInfo(userClaimInfo)

	logrus.Debugf("OpenIDCProvider: loginuser: checking user's access to rancher")
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

	return userPrincipal, groupPrincipals, string(oauthToken), userClaimInfo, err
}

func (o *OpenIDCProvider) SearchPrincipals(searchValue, principalType string, token accessor.TokenAccessor) ([]v3.Principal, error) {
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

func (o *OpenIDCProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error) {
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
	p = o.toPrincipalFromToken(principalType, p, token)
	return p, nil
}

func (o *OpenIDCProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.OIDCProviderFieldRedirectURL] = o.getRedirectURL(authConfig)
	return p, nil
}

func (o *OpenIDCProvider) getRedirectURL(config map[string]interface{}) string {
	authURL, _ := FetchAuthURL(config)

	return fmt.Sprintf(
		"%s?client_id=%s&response_type=code&redirect_uri=%s",
		authURL,
		config["clientId"],
		config["rancherUrl"],
	)
}

func (o *OpenIDCProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	var groupPrincipals []v3.Principal

	config, err := o.GetConfig()
	if err != nil {
		logrus.Errorf("OpenIDCProvider: refetchGroupPrincipals: error fetching OIDCConfig: %v", err)
		return groupPrincipals, err
	}
	// need to get the user information so that the refreshed token can be saved using the username / userID
	user, err := o.UserMGR.GetUserByPrincipalID(principalID)
	if err != nil {
		logrus.Errorf("OpenIDCProvider: refetchGroupPrincipals: error getting user by principalID: %v", err)
		return groupPrincipals, err
	}
	var oauthToken oauth2.Token
	if err := json.Unmarshal([]byte(secret), &oauthToken); err != nil {
		return nil, err
	}

	claimInfo, err := o.getClaimInfoFromToken(o.CTX, config, &oauthToken, user.Name)
	if err != nil {
		return groupPrincipals, err
	}
	return o.getGroupsFromClaimInfo(*claimInfo), nil
}

func (o *OpenIDCProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := o.GetConfig()
	if err != nil {
		logrus.Errorf("OpenIDCProvider: canAccessWithGroupProviders: error fetching OIDCConfig: %v", err)
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

func (o *OpenIDCProvider) toPrincipalFromToken(principalType string, princ v3.Principal, token accessor.TokenAccessor) v3.Principal {
	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = o.IsThisUserMe(token.GetUserPrincipal(), princ)
			if princ.Me {
				tokenPrincipal := token.GetUserPrincipal()
				princ.LoginName = tokenPrincipal.LoginName
				princ.DisplayName = tokenPrincipal.DisplayName
			}
		}
	} else {
		princ.PrincipalType = GroupType
		if token != nil {
			princ.MemberOf = o.TokenMgr.IsMemberOf(token, princ)
		}
	}
	return princ
}

func (o *OpenIDCProvider) saveOIDCConfig(config *v32.OIDCConfig) error {
	storedOidcConfig, err := o.GetConfig()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = o.Type
	config.ObjectMeta = storedOidcConfig.ObjectMeta

	if config.PrivateKey != "" {
		privateKeyField := strings.ToLower(client.OIDCConfigFieldPrivateKey)
		name, err := common.CreateOrUpdateSecrets(o.Secrets, config.PrivateKey, privateKeyField, strings.ToLower(config.Type))
		if err != nil {
			return err
		}
		config.PrivateKey = name
	}

	secretField := strings.ToLower(client.OIDCConfigFieldClientSecret)
	name, err := common.CreateOrUpdateSecrets(o.Secrets, convert.ToString(config.ClientSecret), secretField, strings.ToLower(config.Type))
	if err != nil {
		return err
	}
	config.ClientSecret = name

	logrus.Debugf("OpenIDCProvider: saveOIDCConfig: updating config")
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

func (o *OpenIDCProvider) IsThisUserMe(me, other v3.Principal) bool {
	return common.SamePrincipal(me, other)
}

func (o *OpenIDCProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

func (o *OpenIDCProvider) getUserInfoFromAuthCode(ctx *context.Context, config *v32.OIDCConfig, authCode string, claimInfo *ClaimInfo, userName string) (*oidc.UserInfo, *oauth2.Token, error) {
	var userInfo *oidc.UserInfo
	var oauth2Token *oauth2.Token
	var err error

	updatedContext, err := AddCertKeyToContext(*ctx, config.Certificate, config.PrivateKey)
	if err != nil {
		return userInfo, oauth2Token, err
	}

	provider, err := o.getOIDCProvider(updatedContext, config)
	if err != nil {
		return userInfo, oauth2Token, err
	}
	oauthConfig := ConfigToOauthConfig(provider.Endpoint(), config)
	var verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	oauth2Token, err = oauthConfig.Exchange(updatedContext, authCode, oauth2.SetAuthURLParam("scope", strings.Join(oauthConfig.Scopes, " ")))
	if err != nil {
		return userInfo, oauth2Token, err
	}

	// Get the ID token.  The ID token should be there because we require the openid scope.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return userInfo, oauth2Token, fmt.Errorf("no id_token field in oauth2 token")
	}

	idToken, err := verifier.Verify(updatedContext, rawIDToken)
	if err != nil {
		return userInfo, oauth2Token, fmt.Errorf("failed to verify ID token: %w", err)
	}

	if err := idToken.Claims(&claimInfo); err != nil {
		return userInfo, oauth2Token, fmt.Errorf("failed to parse claims: %w", err)
	}

	// read groups from GroupsClaim if provided
	if config.GroupsClaim != "" {
		groupsClaim, err := getValueFromClaims[[]any](idToken, config.GroupsClaim)
		if err != nil {
			return userInfo, oauth2Token, fmt.Errorf("failed to parse groups claims: %w", err)
		}

		if len(groupsClaim) > 0 {
			logrus.Debugf("OpenIDCProvider: using custom groups claim")
			var groups []string
			for _, g := range groupsClaim {
				group, ok := g.(string)
				if !ok {
					logrus.Warn("OpenIDCProvider: failed to convert group to string")
				}
				groups = append(groups, group)
			}
			claimInfo.Groups = groups
		}
	}

	// read name from nameClaim if provided
	if config.NameClaim != "" {
		nameClaim, err := getValueFromClaims[string](idToken, config.NameClaim)
		if err != nil {
			return userInfo, oauth2Token, fmt.Errorf("failed to parse claims: %w", err)
		}
		claimInfo.Name = nameClaim
	}

	// read email from emailClaim if provided
	if config.EmailClaim != "" {
		emailClaim, err := getValueFromClaims[string](idToken, config.EmailClaim)
		if err != nil {
			return userInfo, oauth2Token, fmt.Errorf("failed to parse claims: %w", err)
		}
		claimInfo.Email = emailClaim
	}

	// Valid will return false if access token is expired
	if !oauth2Token.Valid() {
		return userInfo, oauth2Token, fmt.Errorf("not valid token: %w", err)
	}

	if err := o.UpdateToken(oauth2Token, userName); err != nil {
		return nil, nil, err
	}

	if config.AcrValue != "" {
		acrValue, err := parseACRFromToken(rawIDToken)
		if err != nil {
			return userInfo, oauth2Token, err
		}
		if !isValidACR(acrValue, config.AcrValue) {
			return userInfo, oauth2Token, errors.New("failed to validate ACR")
		}
	}

	logrus.Debugf("OpenIDCProvider: getUserInfo: getting user info for user %s", userName)
	userInfo, err = provider.UserInfo(updatedContext, oauthConfig.TokenSource(updatedContext, oauth2Token))
	if err != nil {
		return userInfo, oauth2Token, err
	}
	if err := userInfo.Claims(&claimInfo); err != nil {
		return userInfo, oauth2Token, err
	}

	return userInfo, oauth2Token, nil
}

func (o *OpenIDCProvider) getClaimInfoFromToken(ctx context.Context, config *v32.OIDCConfig, token *oauth2.Token, userName string) (*ClaimInfo, error) {
	var userInfo *oidc.UserInfo
	var err error
	var claimInfo *ClaimInfo

	updatedContext, err := AddCertKeyToContext(ctx, config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}

	provider, err := o.getOIDCProvider(updatedContext, config)
	if err != nil {
		return nil, err
	}
	oauthConfig := ConfigToOauthConfig(provider.Endpoint(), config)
	var verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	// Valid will return false if access token is expired
	if !token.Valid() {
		// since token is not valid, the TokenSource func will attempt to refresh the access token
		// if the refresh token has not expired
		logrus.Debugf("OpenIDCProvider: getUserInfo: attempting to refresh access token")
		reusedToken, err := oauth2.ReuseTokenSource(token, oauthConfig.TokenSource(updatedContext, token)).Token()
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(token, reusedToken) {
			err := o.UpdateToken(reusedToken, userName)
			if err != nil {
				return nil, fmt.Errorf("failed to update token: %w", err)
			}
		}
		token = reusedToken
	}

	// Try to get the ID token from the oauth2 token first
	rawIDToken, hasIDToken := token.Extra("id_token").(string)

	var idToken *oidc.IDToken

	if hasIDToken {
		// If we have an ID token, use it (this is the correct approach)
		idToken, err = verifier.Verify(updatedContext, rawIDToken)
		if err != nil {
			return nil, fmt.Errorf("failed to verify ID token: %w", err)
		}
	} else {
		// Fallback for backward compatibility - try to verify access token as ID token
		// This handles cases where the token doesn't have ID token in Extra (like some test scenarios)
		idToken, err = verifier.Verify(updatedContext, token.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to verify token: %w", err)
		}
		rawIDToken = token.AccessToken // Use access token for ACR validation if no ID token
	}

	if err := idToken.Claims(&claimInfo); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	if config.AcrValue != "" {
		acrValue, err := parseACRFromToken(rawIDToken)
		if err != nil {
			return nil, err
		}
		if !isValidACR(acrValue, config.AcrValue) {
			return nil, errors.New("failed due to invalid ACR")
		}
	}

	logrus.Debugf("OpenIDCProvider: getUserInfo: getting user info for user %s", userName)
	userInfo, err = provider.UserInfo(updatedContext, oauthConfig.TokenSource(updatedContext, token))
	if err != nil {
		return nil, err
	}
	if err := userInfo.Claims(&claimInfo); err != nil {
		return nil, err
	}

	return claimInfo, nil
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

	// If full_group_path is provided, it takes precedence over groups.
	// full_group_path is expected to be a list of paths separated by '/'.
	// Each path element is treated as a separate group.
	// For example, if full_group_path contains "/group1/group2" and "/group3",
	// the resulting groups will be "group1", "group2", and "group3".
	//
	// This allows for hierarchical group structures to be flattened into individual group memberships.
	if claimInfo.FullGroupPath != nil {
		logrus.Debugf("OpenIDCProvider: using full_group_path claim")
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
		logrus.Debugf("OpenIDCProvider: using groups claim")
		for _, group := range claimInfo.Groups {
			groupPrincipal := o.groupToPrincipal(group)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}

	// If Roles are provided these are added as additional group principals.
	//
	// This is done to support identity providers like Azure AD.
	if claimInfo.Roles != nil {
		logrus.Debugf("OpenIDCProvider: using roles claim")
		for _, role := range claimInfo.Roles {
			groupPrincipal := o.groupToPrincipal(role)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}
	return groupPrincipals
}

func (o *OpenIDCProvider) UpdateToken(refreshedToken *oauth2.Token, userID string) error {
	var err error
	logrus.Debugf("OpenIDCProvider: UpdateToken: access token has been refreshed")
	marshalledToken, err := json.Marshal(refreshedToken)
	if err != nil {
		return err
	}
	logrus.Debugf("OpenIDCProvider: UpdateToken: saving refreshed access token")
	o.TokenMgr.UpdateSecret(userID, o.Name, string(marshalledToken))
	return err
}

// IsDisabledProvider checks if the OIDC auth provider is currently disabled in Rancher.
func (o *OpenIDCProvider) IsDisabledProvider() (bool, error) {
	oidcConfig, err := o.GetConfig()
	if err != nil {
		return false, err
	}
	return !oidcConfig.Enabled, nil
}

func (o *OpenIDCProvider) getOIDCProvider(ctx context.Context, oidcConfig *v32.OIDCConfig) (*oidc.Provider, error) {
	oidcFields := map[string]string{
		client.OIDCConfigFieldIssuer:           oidcConfig.Issuer,
		client.OIDCConfigFieldAuthEndpoint:     oidcConfig.AuthEndpoint,
		client.OIDCConfigFieldTokenEndpoint:    oidcConfig.TokenEndpoint,
		client.OIDCConfigFieldJWKSUrl:          oidcConfig.JWKSUrl,
		client.OIDCConfigFieldUserInfoEndpoint: oidcConfig.UserInfoEndpoint,
	}
	var emptyFields []string
	for key, value := range oidcFields {
		if value == "" {
			emptyFields = append(emptyFields, key)
		}
	}

	// If all the fields are set, we will use them and manually specify each one.
	// Otherwise, we will fall back to using just the issuer and the others will be determined by discovery.
	if len(emptyFields) > 0 && slices.Contains(emptyFields, oidcFields[client.OIDCConfigFieldIssuer]) {
		return nil, fmt.Errorf("unable to create OIDC provider. The following fields are missing: %s", strings.Join(emptyFields, ","))
	}

	if len(emptyFields) == 0 {
		pConfig := &oidc.ProviderConfig{
			IssuerURL:   oidcConfig.Issuer,
			AuthURL:     oidcConfig.AuthEndpoint,
			TokenURL:    oidcConfig.TokenEndpoint,
			UserInfoURL: oidcConfig.UserInfoEndpoint,
			JWKSURL:     oidcConfig.JWKSUrl,
		}
		return pConfig.NewProvider(ctx), nil
	}
	// This will perform discovery in the oidc library
	return oidc.NewProvider(ctx, oidcConfig.Issuer)
}

func (o *OpenIDCProvider) Logout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	providerName := token.GetAuthProvider()
	logrus.Debugf("OpenIDCProvider [logout]: triggered by provider %s", providerName)
	oidcConfig, err := o.GetConfig()
	if err != nil {
		return fmt.Errorf("getting config for OIDC Logout: %w", err)
	}
	if oidcConfig.LogoutAllForced {
		logrus.Debugf("OpenIDCProvider [logout]: Rancher provider resource `%v` configured for forced SLO, rejecting regular logout", providerName)
		return fmt.Errorf("OpenIDCProvider [logout]: Rancher provider resource `%v` configured for forced SLO, rejecting regular logout", providerName)
	}

	return nil
}

func (o *OpenIDCProvider) LogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	logrus.Debugf("OpenIDCProvider [logout-all]: triggered by provider %s", token.GetAuthProvider())

	oidcConfig, err := o.GetConfig()
	if err != nil {
		return err
	}

	providerName := token.GetAuthProvider()
	if !oidcConfig.LogoutAllEnabled {
		logrus.Debugf("OpenIDCProvider [logout-all]: Rancher provider resource `%v` not configured for SLO", providerName)
		return fmt.Errorf("OpenIDCProvider [logout-all]: Rancher provider resource `%v` not configured for SLO", providerName)
	}

	idpRedirectURL, err := o.createIDPRedirectURL(apiContext, oidcConfig)
	if err != nil {
		return err
	}
	logrus.Debug("OpenIDCProvider [logout-all]: triggering logout redirect to ", idpRedirectURL)

	data := map[string]interface{}{
		"idpRedirectUrl": idpRedirectURL,
		"type":           "authConfigLogoutOutput",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	logrus.Debug("OpenIDCProvider [logout-all]: redirect written")

	return nil
}

func (o *OpenIDCProvider) createIDPRedirectURL(apiContext *types.APIContext, config *v32.OIDCConfig) (string, error) {
	if config.EndSessionEndpoint == "" {
		return "", httperror.NewAPIError(httperror.ServerError, "LogoutAll triggered with no endSessionEndpoint")
	}

	idpRedirectURL, err := url.Parse(config.EndSessionEndpoint)
	if err != nil {
		logrus.Infof("OpenIDCProvider: [logout-all] failed parsing end session endpoint: %v", err)
		return "", err
	}

	authLogout := &v32.AuthConfigLogoutInput{}
	if err := json.NewDecoder(apiContext.Request.Body).Decode(authLogout); err != nil {
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("OIDC: parsing request body: %v", err))
	}

	// https://openid.net/specs/openid-connect-rpinitiated-1_0.html#rfc.section.2
	params := idpRedirectURL.Query()
	// If there's no post_logout_redirect_uri then it will redirect to the
	// redirect_uri for the client ID.
	if authLogout.FinalRedirectURL != "" {
		params.Set("post_logout_redirect_uri", authLogout.FinalRedirectURL)
	}

	// This triggers logout without an id_token_hint.
	params.Set("client_id", config.ClientID)

	idpRedirectURL.RawQuery = params.Encode()

	return idpRedirectURL.String(), nil
}

func isValidACR(claimACR string, configuredACR string) bool {
	// if we have no ACR configured, all values are accepted
	if configuredACR == "" {
		return true
	}

	if claimACR != configuredACR {
		logrus.Infof("OpenIDCProvider: acr value in token does not match configured acr value")
		return false
	}
	return true
}

func parseACRFromToken(token string) (string, error) {
	var parser jwt.Parser
	// we already validated the incoming token
	parsedToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT token: %w", err)
	}
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("failed to parse claims in JWT token: invalid jwt.MapClaims format")
	}

	acrValue, found := claims["acr"].(string)
	if !found {
		return "", fmt.Errorf("ACR claim invalid or not found in token: (acr=%v)", claims["acr"])
	}

	return acrValue, nil
}

func getValueFromClaims[T any](idToken *oidc.IDToken, name string) (T, error) {
	var mapClaims jwt.MapClaims
	err := idToken.Claims(&mapClaims)
	if err != nil {
		var empty T
		return empty, fmt.Errorf("failed to parse claims: %w", err)
	}
	claim, ok := mapClaims[name].(T)
	if !ok {
		logrus.Warnf("failed to use custom %s claim", name)
	} else {
		logrus.Debugf("using custom %s claim", name)
	}

	return claim, nil
}
