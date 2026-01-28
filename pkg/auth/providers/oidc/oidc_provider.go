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
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name                   = "oidc"
	UserType               = "user"
	GroupType              = "group"
	IDTokenCookie          = "R_OIDC_ID"
	pkceVerifierCookieName = "R_PKCE_VERIFIER"

	// PKCES256 is a constant for the SHA256 PKCE Verification method.
	PKCES256Method string = "S256"
)

type tokenManager interface {
	UpdateSecret(userID, provider, secret string) error
	CreateTokenAndSetCookie(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error
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
	GetConfig   func() (*apiv3.OIDCConfig, error)
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

func (o *OpenIDCProvider) AuthenticateUser(w http.ResponseWriter, req *http.Request, input any) (apiv3.Principal, []apiv3.Principal, string, error) {
	login, ok := input.(*apiv3.OIDCLogin)
	if !ok {
		return apiv3.Principal{}, nil, "", fmt.Errorf("unexpected input type")
	}
	userPrincipal, groupPrincipals, providerToken, _, err := o.LoginUser(w, req, login, nil)
	return userPrincipal, groupPrincipals, providerToken, err
}

func (o *OpenIDCProvider) LoginUser(w http.ResponseWriter, req *http.Request, oauthLoginInfo *apiv3.OIDCLogin, config *apiv3.OIDCConfig) (apiv3.Principal, []apiv3.Principal, string, ClaimInfo, error) {
	var userPrincipal apiv3.Principal
	var groupPrincipals []apiv3.Principal
	var userClaimInfo ClaimInfo
	var err error

	if config == nil {
		config, err = o.GetConfig()
		if err != nil {
			return userPrincipal, nil, "", userClaimInfo, err
		}
	}

	userInfo, oauth2Token, idToken, err := o.getUserInfoFromAuthCode(w, req, config, oauthLoginInfo.Code, &userClaimInfo, "")
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	userPrincipal = o.userToPrincipal(userInfo, userClaimInfo)
	userPrincipal.Me = true
	groupPrincipals = o.getGroupsFromClaimInfo(userClaimInfo)

	logrus.Debug("OpenIDCProvider: loginuser: checking user's access to rancher")
	allowed, err := o.UserMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	if !allowed {
		return userPrincipal, groupPrincipals, "", userClaimInfo, apierror.NewAPIError(validation.Unauthorized, "unauthorized")
	}
	// save entire oauthToken because it contains refresh_token and token expiry time
	// will use with oauth2.Client and with TokenSource to ensure auto refresh of tokens occurs for api calls
	oauthToken, err := json.Marshal(oauth2Token)
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}

	setIDToken(req, w, idToken)
	return userPrincipal, groupPrincipals, string(oauthToken), userClaimInfo, err
}

func (o *OpenIDCProvider) SearchPrincipals(searchValue, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	var principals []apiv3.Principal

	if principalType == "" {
		principalType = UserType
	}

	p := apiv3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: o.Name + "_" + principalType + "://" + searchValue},
		DisplayName:   searchValue,
		LoginName:     searchValue,
		PrincipalType: principalType,
		Provider:      o.Name,
	}

	principals = append(principals, p)
	return principals, nil
}

func (o *OpenIDCProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	var p apiv3.Principal

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
		p = apiv3.Principal{
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

func (o *OpenIDCProvider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	p := common.TransformToAuthProvider(authConfig)
	redirectPath, err := o.getRedirectURL(authConfig)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC login URL: %w", err)
	}

	p[publicclient.OIDCProviderFieldRedirectURL] = redirectPath

	return p, nil
}

type urlValues interface {
	Add(_, _ string)
	Encode() string
}

// GetOIDCRedirectionURL generates the URL to redirect to the provider.
//
// the Values can either be an `orderedValues` value or a url.Values.
func GetOIDCRedirectionURL(config map[string]any, pkceVerifier string, values urlValues) string {
	authURL, _ := FetchAuthURL(config)

	values.Add("client_id", config["clientId"].(string))
	values.Add("response_type", "code")

	logrus.Debug("Checking for PKCE")
	if pkceMethod, ok := config[client.GenericOIDCConfigFieldPKCEMethod]; ok {
		if pkceVerifier != "" {
			switch pkceMethod {
			case PKCES256Method:
				logrus.Debug("PKCE Enabled sending code_challenge and s256 code_challenge_method")
				values.Add("code_challenge", oauth2.S256ChallengeFromVerifier(pkceVerifier))
				values.Add("code_challenge_method", PKCES256Method)
			default:
				logrus.Debug("PKCE NOT Enabled for redirect URL")
			}
		} else {
			logrus.Debugf("PKCE requested but no verifier available")
		}
	} else {
		logrus.Debug("PKCE - no configuration")
	}

	values.Add("redirect_uri", config["rancherUrl"].(string))

	return fmt.Sprintf("%s?%s", authURL, values.Encode())
}

func (o *OpenIDCProvider) getRedirectURL(authConfig map[string]any) (string, error) {
	name := authConfig["metadata"].(map[string]any)["name"].(string)
	rancherAPIHost, ok := authConfig[client.GenericOIDCConfigFieldRancherAPIHost].(string)
	// No API Host - no PKCE Redirection.
	if !ok {
		logrus.Debugf("OpenIDCProvider: No API Host - no PKCE Redirection")
		authURL, _ := FetchAuthURL(authConfig)
		return fmt.Sprintf(
			"%s?client_id=%s&response_type=code&redirect_uri=%s",
			authURL,
			authConfig["clientId"],
			authConfig["rancherUrl"],
		), nil

	}

	// This redirects via the handler in pkg/auth/handler
	return url.JoinPath(rancherAPIHost, "v1-oidc", name)
}

func (o *OpenIDCProvider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	var groupPrincipals []apiv3.Principal

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

func (o *OpenIDCProvider) userToPrincipal(userInfo *oidc.UserInfo, claimInfo ClaimInfo) apiv3.Principal {
	displayName := claimInfo.Name
	if displayName == "" {
		displayName = userInfo.Email
	}
	p := apiv3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: o.Name + "_" + UserType + "://" + userInfo.Subject},
		DisplayName:   displayName,
		LoginName:     userInfo.Email,
		Provider:      o.Name,
		PrincipalType: UserType,
		Me:            false,
	}
	return p
}

func (o *OpenIDCProvider) groupToPrincipal(groupName string) apiv3.Principal {
	p := apiv3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: o.Name + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		Provider:      o.Name,
		PrincipalType: GroupType,
		Me:            false,
	}
	return p
}

func (o *OpenIDCProvider) toPrincipalFromToken(principalType string, princ apiv3.Principal, token accessor.TokenAccessor) apiv3.Principal {
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
			princ.MemberOf = o.UserMGR.IsMemberOf(token, princ)
		}
	}
	return princ
}

func (o *OpenIDCProvider) saveOIDCConfig(config *apiv3.OIDCConfig) error {
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

func (o *OpenIDCProvider) GetOIDCConfig() (*apiv3.OIDCConfig, error) {
	authConfigObj, err := o.AuthConfigs.ObjectClient().UnstructuredClient().Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, cannot read k8s Unstructured data")
	}
	storedOidcConfigMap := u.UnstructuredContent()

	storedOidcConfig := &apiv3.OIDCConfig{}
	err = common.Decode(storedOidcConfigMap, storedOidcConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode OidcConfig: %w", err)
	}
	if storedOidcConfig.PKCEMethod != "" {
		logrus.Debugf("GetOIDCConfig PKCE Enabled for %s", o.Name)
	} else {
		logrus.Debugf("GetOIDCConfig PKCE IS NOT Enabled %s", o.Name)
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

func (o *OpenIDCProvider) IsThisUserMe(me, other apiv3.Principal) bool {
	return common.SamePrincipal(me, other)
}

func (o *OpenIDCProvider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

func (o *OpenIDCProvider) getUserInfoFromAuthCode(rw http.ResponseWriter, req *http.Request, config *apiv3.OIDCConfig, authCode string, claimInfo *ClaimInfo, userName string) (*oidc.UserInfo, *oauth2.Token, string, error) {
	var userInfo *oidc.UserInfo
	var oauth2Token *oauth2.Token
	var err error

	updatedContext, err := AddCertKeyToContext(req.Context(), config.Certificate, config.PrivateKey)
	if err != nil {
		return userInfo, oauth2Token, "", err
	}

	provider, err := o.getOIDCProvider(updatedContext, config)
	if err != nil {
		return userInfo, oauth2Token, "", err
	}
	oauthConfig := ConfigToOauthConfig(provider.Endpoint(), config)
	var verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("scope", strings.Join(oauthConfig.Scopes, " ")),
	}

	if config.PKCEMethod != "" {
		pkceVerifier := getPKCEVerifier(req)
		if pkceVerifier != "" {
			logrus.Debug("OpenIDCProvider: PKCE Enabled - sending verifier in token exchange")
			opts = append(opts, oauth2.VerifierOption(pkceVerifier))
			// We can delete the token as even if it fails, it will require a new
			// token.
			logrus.Debugf("OpenIDCProvider: PKCE Enabled - deleting the cookie")
			deletePKCEVerifier(req, rw)
		} else {
			logrus.Debug("OpenIDCProvider: PKCE Enabled - but no cookie was available for verifier")
		}
	} else {
		logrus.Debug("OpenIDCProvider: PKCE not Enabled - not sending verifier")
	}

	oauth2Token, err = oauthConfig.Exchange(updatedContext, authCode, opts...)
	if err != nil {
		return userInfo, oauth2Token, "", fmt.Errorf("failed exchanging token: %w", err)
	}

	// Get the ID token.  The ID token should be there because we require the openid scope.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return userInfo, oauth2Token, "", fmt.Errorf("no id_token field in oauth2 token")
	}

	idToken, err := verifier.Verify(updatedContext, rawIDToken)
	if err != nil {
		return userInfo, oauth2Token, "", fmt.Errorf("failed to verify ID token: %w", err)
	}

	if err := idToken.Claims(&claimInfo); err != nil {
		return userInfo, oauth2Token, "", fmt.Errorf("failed to parse claims: %w", err)
	}

	// read groups from GroupsClaim if provided
	if config.GroupsClaim != "" {
		groupsClaim, err := getValueFromClaims[[]any](idToken, config.GroupsClaim)
		if err != nil {
			return userInfo, oauth2Token, "", fmt.Errorf("failed to parse groups claims: %w", err)
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
			return userInfo, oauth2Token, "", fmt.Errorf("failed to parse claims: %w", err)
		}
		claimInfo.Name = nameClaim
	}

	// read email from emailClaim if provided
	if config.EmailClaim != "" {
		emailClaim, err := getValueFromClaims[string](idToken, config.EmailClaim)
		if err != nil {
			return userInfo, oauth2Token, "", fmt.Errorf("failed to parse claims: %w", err)
		}
		claimInfo.Email = emailClaim
	}

	// Valid will return false if access token is expired
	if !oauth2Token.Valid() {
		return userInfo, oauth2Token, "", fmt.Errorf("not valid token: %w", err)
	}

	if err := o.UpdateToken(oauth2Token, userName); err != nil {
		return nil, nil, "", err
	}

	if config.AcrValue != "" {
		acrValue, err := parseACRFromAccessToken(oauth2Token.AccessToken)
		if err != nil {
			return userInfo, oauth2Token, "", err
		}
		if !isValidACR(acrValue, config.AcrValue) {
			return userInfo, oauth2Token, "", errors.New("failed to validate ACR")
		}
	}

	logrus.Debugf("OpenIDCProvider: getUserInfo: getting user info for user %s", userName)
	userInfo, err = provider.UserInfo(updatedContext, oauthConfig.TokenSource(updatedContext, oauth2Token))
	if err != nil {
		return userInfo, oauth2Token, "", err
	}

	return userInfo, oauth2Token, rawIDToken, nil
}

func (o *OpenIDCProvider) getClaimInfoFromToken(ctx context.Context, config *apiv3.OIDCConfig, token *oauth2.Token, userName string) (*ClaimInfo, error) {
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

	idToken, err := verifier.Verify(updatedContext, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}
	if err := idToken.Claims(&claimInfo); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	if config.AcrValue != "" {
		acrValue, err := parseACRFromAccessToken(token.AccessToken)
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

func ConfigToOauthConfig(endpoint oauth2.Endpoint, config *apiv3.OIDCConfig) oauth2.Config {
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

func (o *OpenIDCProvider) getGroupsFromClaimInfo(claimInfo ClaimInfo) []apiv3.Principal {
	var groupPrincipals []apiv3.Principal

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

func (o *OpenIDCProvider) getOIDCProvider(ctx context.Context, oidcConfig *apiv3.OIDCConfig) (*oidc.Provider, error) {
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

func (o *OpenIDCProvider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
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

func (o *OpenIDCProvider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
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

	idpRedirectURL, err := o.createIDPRedirectURL(r, oidcConfig)
	if err != nil {
		return err
	}
	logrus.Debug("OpenIDCProvider [logout-all]: triggering logout redirect to ", idpRedirectURL)

	data := map[string]any{
		"idpRedirectUrl": idpRedirectURL,
		"type":           "authConfigLogoutOutput",
		"baseType":       "authConfigLogoutOutput",
	}

	logrus.Debug("OpenIDCProvider [logout-all]: writing redirect")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

func (o *OpenIDCProvider) createIDPRedirectURL(r *http.Request, config *apiv3.OIDCConfig) (string, error) {
	if config.EndSessionEndpoint == "" {
		return "", httperror.NewAPIError(httperror.ServerError, "LogoutAll triggered with no endSessionEndpoint")
	}

	idpRedirectURL, err := url.Parse(config.EndSessionEndpoint)
	if err != nil {
		logrus.Errorf("OpenIDCProvider: [logout-all] failed parsing end session endpoint: %v", err)
		return "", err
	}

	authLogout := &apiv3.AuthConfigLogoutInput{}
	if err := json.NewDecoder(r.Body).Decode(authLogout); err != nil {
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("OIDC: parsing request body: %v", err))
	}

	// https://openid.net/specs/openid-connect-rpinitiated-1_0.html#rfc.section.2
	params := idpRedirectURL.Query()
	// If there's no post_logout_redirect_uri then it will redirect to the
	// redirect_uri for the client ID.
	if authLogout.FinalRedirectURL != "" {
		params.Set("post_logout_redirect_uri", authLogout.FinalRedirectURL)
		logrus.Debugf("OpenIDCProvider: [logout-all] redirecting to %s", authLogout.FinalRedirectURL)
	}

	idToken := getIDToken(r)
	// Not sending the ID token is allowed - this means that there has to be a
	// session between the user's browser and the OP and this should be
	// terminated.
	if idToken != "" {
		params.Set("id_token_hint", idToken)
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

func parseACRFromAccessToken(accessToken string) (string, error) {
	var parser jwt.Parser
	// we already validated the incoming token
	token, _, err := parser.ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
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
		logrus.Debugf("OpenIDCProvider: failed to use claim %v", name)
	} else {
		logrus.Debugf("OpenIDCProvider: using claim %v", name)
	}

	return claim, nil
}

func deletePKCEVerifier(req *http.Request, w http.ResponseWriter) {
	isSecure := req.URL.Scheme == "https"
	pkceCookie := &http.Cookie{
		Name:    pkceVerifierCookieName,
		Value:   "",
		Secure:  isSecure,
		Expires: time.Now().Add(time.Second * -10),
	}

	http.SetCookie(w, pkceCookie)
}

// SetPKCEVerifier sets Cookie with the PKCE Verification token which is needed
// when the user's browser is redirected back to exchange the token.
func SetPKCEVerifier(req *http.Request, w http.ResponseWriter, value string) {
	isSecure := req.URL.Scheme == "https"
	pkceCookie := &http.Cookie{
		Name:     pkceVerifierCookieName,
		Value:    value,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(time.Minute * 10),
	}

	http.SetCookie(w, pkceCookie)
}

func setIDToken(req *http.Request, w http.ResponseWriter, token string) {
	isSecure := req.URL.Scheme == "https"
	tokenCookie := &http.Cookie{
		Name:     IDTokenCookie,
		Value:    token,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, tokenCookie)
}

func getIDToken(req *http.Request) string {
	cookie, err := req.Cookie(IDTokenCookie)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func getPKCEVerifier(req *http.Request) string {
	cookie, err := req.Cookie(pkceVerifierCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// This is used instead of url.Values to avoid URL encoding the values.
//
// It implements a subset of the url.Values struct.
type orderedValues []string

// this will take a slice of pairs of strings and generate a non-encoded string
// in the order they are added separated by &
// e.g. []string{"testing","value","user","1"} == "testing=value&user=1"
//
// If an uneven number of elements is passed, the final one will be dropped.
func (v orderedValues) Encode() string {
	if len(v) == 0 {
		return ""
	}

	var buf strings.Builder
	for i := 0; i < len(v); i += 2 {
		if len(v)-i < 2 {
			break
		}
		k, v := v[i], v[i+1]
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
	}

	return buf.String()
}

func (v *orderedValues) Add(key, value string) {
	*v = append(*v, key, value)
}
