package genericoidc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type GenericOIDCProvider struct {
	baseoidc.OpenIDCProvider
}

const (
	Name      = "genericoidc"
	UserType  = "user"
	GroupType = "group"
)

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
	return &GenericOIDCProvider{
		baseoidc.OpenIDCProvider{
			Name:        Name,
			Type:        client.GenericOIDCConfigType,
			CTX:         ctx,
			AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
			Secrets:     mgmtCtx.Core.Secrets(""),
			UserMGR:     userMGR,
			TokenMGR:    tokenMGR,
		},
	}
}

func (g *GenericOIDCProvider) GetName() string {
	return Name
}

// SearchPrincipals will return a principal of the requested principalType with a displayName and loginName
// that match the searchValue.  This is done because OIDC does not have a proper lookup mechanism.  In order
// to provide some degree of functionality that allows manual entry for users/groups, this is the compromise.
func (g *GenericOIDCProvider) SearchPrincipals(searchValue, principalType string, _ v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal

	if principalType == "" {
		principalType = UserType
	}

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + principalType + "://" + searchValue},
		DisplayName:   searchValue,
		LoginName:     searchValue,
		PrincipalType: principalType,
		Provider:      g.Name,
	}

	principals = append(principals, p)
	return principals, nil
}

func (g *GenericOIDCProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	var p v3.Principal

	// parsing id to get the external id and type. Example genericoidc_<user|group>://<user sub | group name>
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
	provider := parts[0]
	principalType := parts[1]
	if externalID == "" && principalType == "" {
		return p, fmt.Errorf("invalid id %v", principalID)
	}
	if principalType != UserType && principalType != GroupType {
		return p, fmt.Errorf("invalid principal type")
	}
	if principalType == UserType {
		p = v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: provider + "_" + principalType + "://" + externalID},
			DisplayName:   externalID,
			LoginName:     externalID,
			PrincipalType: UserType,
			Provider:      g.Name,
		}
	} else {
		p = g.groupToPrincipal(externalID)
	}
	p = g.toPrincipalFromToken(principalType, p, &token)
	return p, nil
}

func (g *GenericOIDCProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.GenericOIDCProviderFieldRedirectURL] = g.getRedirectURL(authConfig)
	return p, nil
}

// RefetchGroupPrincipals is not implemented for OIDC.  The typical lifespan of a refresh token (minutes, not hours or days)
// would not grant the functionality that we require.
func (g *GenericOIDCProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return nil, errors.New("Not implemented")
}

func (g *GenericOIDCProvider) GetOIDCConfig() (*v32.OIDCConfig, error) {
	authConfigObj, err := g.AuthConfigs.ObjectClient().UnstructuredClient().Get(g.Name, metav1.GetOptions{})
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
		value, err := common.ReadFromSecret(g.Secrets, storedOidcConfig.PrivateKey, strings.ToLower(client.GenericOIDCConfigFieldPrivateKey))
		if err != nil {
			return nil, err
		}
		storedOidcConfig.PrivateKey = value
	}
	if storedOidcConfig.ClientSecret != "" {
		data, err := common.ReadFromSecretData(g.Secrets, storedOidcConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for _, v := range data {
			storedOidcConfig.ClientSecret = string(v)
		}
	}

	return storedOidcConfig, nil
}

func (g *GenericOIDCProvider) UpdateToken(refreshedToken *oauth2.Token, userID string) error {
	var err error
	logrus.Debugf("[generic oidc] UpdateToken: access token has been refreshed")
	marshalledToken, err := json.Marshal(refreshedToken)
	if err != nil {
		return err
	}
	logrus.Debugf("[generic oidc] UpdateToken: saving refreshed access token")
	g.TokenMGR.UpdateSecret(userID, g.Name, string(marshalledToken))
	return err
}

func (g *GenericOIDCProvider) userToPrincipal(userInfo *oidc.UserInfo, claimInfo ClaimInfo) v3.Principal {
	displayName := claimInfo.Name
	if displayName == "" {
		displayName = userInfo.Email
	}
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + UserType + "://" + userInfo.Subject},
		DisplayName:   displayName,
		LoginName:     userInfo.Email,
		Provider:      g.Name,
		PrincipalType: UserType,
		Me:            false,
	}
	return p
}

func (g *GenericOIDCProvider) getGroupsFromClaimInfo(claimInfo ClaimInfo) []v3.Principal {
	var groupPrincipals []v3.Principal

	if claimInfo.FullGroupPath != nil {
		for _, groupPath := range claimInfo.FullGroupPath {
			groupsFromPath := strings.Split(groupPath, "/")
			for _, group := range groupsFromPath {
				if group != "" {
					groupPrincipal := g.groupToPrincipal(group)
					groupPrincipal.MemberOf = true
					groupPrincipals = append(groupPrincipals, groupPrincipal)
				}
			}
		}
	} else {
		for _, group := range claimInfo.Groups {
			groupPrincipal := g.groupToPrincipal(group)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}
	return groupPrincipals
}

func (g *GenericOIDCProvider) groupToPrincipal(groupName string) v3.Principal {
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		Provider:      g.Name,
		PrincipalType: GroupType,
		Me:            false,
	}
	return p
}

func (g *GenericOIDCProvider) getRedirectURL(config map[string]interface{}) string {
	// TODO maybe use discovery in case authurl isn't present...might not be needed if authendpoint is already set in config

	authURL, _ := baseoidc.FetchAuthURL(config["issuer"].(string))

	return fmt.Sprintf(
		"%s?client_id=%s&response_type=code&redirect_uri=%s",
		authURL,
		config["clientId"],
		config["rancherUrl"],
	)
}

func (g *GenericOIDCProvider) toPrincipalFromToken(principalType string, princ v3.Principal, token *v3.Token) v3.Principal {
	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = g.IsThisUserMe(token.UserPrincipal, princ)
			if princ.Me {
				princ.LoginName = token.UserPrincipal.LoginName
				princ.DisplayName = token.UserPrincipal.DisplayName
			}
		}
	} else {
		princ.PrincipalType = GroupType
		if token != nil {
			princ.MemberOf = g.TokenMGR.IsMemberOf(*token, princ)
		}
	}
	return princ
}

func (g *GenericOIDCProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v32.GenericOIDCLogin)
	if !ok {
		return v3.Principal{}, nil, "", fmt.Errorf("unexpected input type")
	}
	userPrincipal, groupPrincipals, providerToken, _, err := g.LoginUser(ctx, login, nil)
	return userPrincipal, groupPrincipals, providerToken, err
}

func (g *GenericOIDCProvider) LoginUser(ctx context.Context, oauthLoginInfo *v32.GenericOIDCLogin, config *v32.OIDCConfig) (v3.Principal, []v3.Principal, string, ClaimInfo, error) {
	var userPrincipal v3.Principal
	var groupPrincipals []v3.Principal
	var userClaimInfo ClaimInfo
	var err error

	if config == nil {
		config, err = g.GetOIDCConfig()
		if err != nil {
			return userPrincipal, nil, "", userClaimInfo, err
		}
	}
	userInfo, oauth2Token, err := g.getUserInfo(&ctx, config, oauthLoginInfo.Code, &userClaimInfo, "")
	if err != nil {
		return userPrincipal, groupPrincipals, "", userClaimInfo, err
	}
	userPrincipal = g.userToPrincipal(userInfo, userClaimInfo)
	userPrincipal.Me = true
	groupPrincipals = g.getGroupsFromClaimInfo(userClaimInfo)

	logrus.Debugf("[generic oidc] loginuser: checking user's access to rancher")
	allowed, err := g.UserMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal.Name, groupPrincipals)
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

func (g *GenericOIDCProvider) getUserInfo(ctx *context.Context, config *v32.OIDCConfig, authCode string, claimInfo *ClaimInfo, userName string) (*oidc.UserInfo, *oauth2.Token, error) {
	var userInfo *oidc.UserInfo
	var oauth2Token *oauth2.Token
	var err error

	updatedContext, err := baseoidc.AddCertKeyToContext(*ctx, config.Certificate, config.PrivateKey)
	if err != nil {
		return userInfo, oauth2Token, err
	}

	provider, err := oidc.NewProvider(updatedContext, config.Issuer)
	if err != nil {
		return userInfo, oauth2Token, err
	}
	oauthConfig := baseoidc.ConfigToOauthConfig(provider.Endpoint(), config)
	var verifier = provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	if err := json.Unmarshal([]byte(authCode), &oauth2Token); err != nil {
		oauth2Token, err = oauthConfig.Exchange(updatedContext, authCode, oauth2.SetAuthURLParam("scope", strings.Join(oauthConfig.Scopes, " ")))
		if err != nil {
			return userInfo, oauth2Token, err
		}
		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		_, err = verifier.Verify(updatedContext, rawIDToken)
		if !ok {
			return userInfo, oauth2Token, err
		}
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
		g.UpdateToken(reusedToken, userName)
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
