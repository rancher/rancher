package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/cognito"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/keycloakoidc"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	"github.com/rancher/rancher/pkg/types/config"
)

const (
	LocalProviderType           = "localProvider"
	GithubProviderType          = "githubProvider"
	AzureADProviderType         = "azureADProvider"
	ActiveDirectoryProviderType = "activeDirectoryProvider"
	OpenLdapProviderType        = "openLdapProvider"
	FreeIpaProviderType         = "freeIpaProvider"
	PingProviderType            = "pingProvider"
	ADFSProviderType            = "adfsProvider"
	KeyCloakProviderType        = "keyCloakProvider"
	OKTAProviderType            = "oktaProvider"
	ShibbolethProviderType      = "shibbolethProvider"
	GoogleOAuthProviderType     = "googleOAuthProvider"
	OIDCProviderType            = "oidcProvider"
	KeyCloakOIDCProviderType    = "keyCloakOIDCProvider"
	GenericOIDCProviderType     = "genericOIDCProvider"
	CognitoProviderType         = "cognitoProvider"
)

const (
	LocalProviderName           = local.Name
	GithubProviderName          = github.Name
	AzureADProviderName         = azure.Name
	ActiveDirectoryProviderName = activedirectory.Name
	OpenLdapProviderName        = ldap.OpenLdapName
	FreeIpaProviderName         = ldap.FreeIpaName
	PingProviderName            = saml.PingName
	ADFSProviderName            = saml.ADFSName
	KeyCloakProviderName        = saml.KeyCloakName
	OKTAProviderName            = saml.OKTAName
	ShibbolethProviderName      = saml.ShibbolethName
	GoogleOAuthProviderName     = googleoauth.Name
	OIDCProviderName            = oidc.Name
	KeyCloakOIDCProviderName    = keycloakoidc.Name
	GenericOIDCProviderName     = genericoidc.Name
	CognitoProviderName         = cognito.Name
)

var (
	ProviderNames          = make(map[string]bool)
	providersWithSecrets   = make(map[string]bool)
	UnrefreshableProviders = make(map[string]bool)
	Providers              = make(map[string]common.AuthProvider)
	LocalProvider          = "local"
	providersByType        = make(map[string]common.AuthProvider)
	confMu                 sync.Mutex
	userExtraAttributesMap = map[string]bool{common.UserAttributePrincipalID: true, common.UserAttributeUserName: true}
)

func IsSAMLProviderType(providerType string) bool {
	switch providerType {
	case PingProviderType, ADFSProviderType, KeyCloakProviderType, OKTAProviderType, ShibbolethProviderType:
		return true
	default:
		return false
	}
}

func IsSAMLProviderName(providerName string) bool {
	switch providerName {
	case PingProviderName, ADFSProviderName, KeyCloakProviderName, OKTAProviderName, ShibbolethProviderName:
		return true
	default:
		return false
	}
}

func GetProvider(providerName string) (common.AuthProvider, error) {
	if provider, ok := Providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no such provider '%s'", providerName)
}

func GetProviderByType(configType string) common.AuthProvider {
	return providersByType[configType]
}

func Configure(ctx context.Context, mgmt *config.ScaledContext) {
	confMu.Lock()
	defer confMu.Unlock()

	userMGR := mgmt.UserManager
	tokenMGR := tokens.NewManager(mgmt.Wrangler)

	// TODO: refactor to eliminate the need for these callbacks, which exist to avoid the import cycle.
	tokens.OnLogoutAll(ProviderLogoutAll)
	tokens.OnLogout(ProviderLogout)

	var p common.AuthProvider

	p = local.Configure(ctx, mgmt, userMGR)
	ProviderNames[local.Name] = true
	Providers[local.Name] = p
	providersByType[client.LocalConfigType] = p
	providersByType[publicclient.LocalProviderType] = p

	p = github.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[github.Name] = true
	providersWithSecrets[github.Name] = true
	Providers[github.Name] = p
	providersByType[client.GithubConfigType] = p
	providersByType[publicclient.GithubProviderType] = p

	p = githubapp.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[githubapp.Name] = true
	providersWithSecrets[githubapp.Name] = true
	Providers[githubapp.Name] = p
	providersByType[client.GithubAppConfigType] = p
	providersByType[publicclient.GithubAppProviderType] = p

	p = azure.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[azure.Name] = true
	providersWithSecrets[azure.Name] = true
	Providers[azure.Name] = p
	providersByType[client.AzureADConfigType] = p
	providersByType[publicclient.AzureADProviderType] = p

	p = activedirectory.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[activedirectory.Name] = true
	Providers[activedirectory.Name] = p
	providersByType[client.ActiveDirectoryConfigType] = p
	providersByType[publicclient.ActiveDirectoryProviderType] = p

	p = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.OpenLdapName)
	ProviderNames[ldap.OpenLdapName] = true
	Providers[ldap.OpenLdapName] = p
	providersByType[client.OpenLdapConfigType] = p
	providersByType[publicclient.OpenLdapProviderType] = p

	p = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.FreeIpaName)
	ProviderNames[ldap.FreeIpaName] = true
	Providers[ldap.FreeIpaName] = p
	providersByType[client.FreeIpaConfigType] = p
	providersByType[publicclient.FreeIpaProviderType] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.PingName)
	ProviderNames[saml.PingName] = true
	UnrefreshableProviders[saml.PingName] = true
	Providers[saml.PingName] = p
	providersByType[client.PingConfigType] = p
	providersByType[publicclient.PingProviderType] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.ADFSName)
	ProviderNames[saml.ADFSName] = true
	UnrefreshableProviders[saml.ADFSName] = true
	Providers[saml.ADFSName] = p
	providersByType[client.ADFSConfigType] = p
	providersByType[publicclient.ADFSProviderType] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	ProviderNames[saml.KeyCloakName] = true
	UnrefreshableProviders[saml.KeyCloakName] = true
	Providers[saml.KeyCloakName] = p
	providersByType[client.KeyCloakConfigType] = p
	providersByType[publicclient.KeyCloakProviderType] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.OKTAName)
	ProviderNames[saml.OKTAName] = true
	UnrefreshableProviders[saml.OKTAName] = true
	Providers[saml.OKTAName] = p
	providersByType[client.OKTAConfigType] = p
	providersByType[publicclient.OKTAProviderType] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.ShibbolethName)
	ProviderNames[saml.ShibbolethName] = true
	UnrefreshableProviders[saml.ShibbolethName] = false
	Providers[saml.ShibbolethName] = p
	providersByType[client.ShibbolethConfigType] = p
	providersByType[publicclient.ShibbolethProviderType] = p

	p = googleoauth.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[googleoauth.Name] = true
	providersWithSecrets[googleoauth.Name] = true
	Providers[googleoauth.Name] = p
	providersByType[client.GoogleOauthConfigType] = p
	providersByType[publicclient.GoogleOAuthProviderType] = p

	p = oidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[oidc.Name] = true
	providersWithSecrets[oidc.Name] = true
	Providers[oidc.Name] = p
	providersByType[client.OIDCConfigType] = p
	providersByType[publicclient.OIDCProviderType] = p

	p = keycloakoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[keycloakoidc.Name] = true
	providersWithSecrets[keycloakoidc.Name] = true
	Providers[keycloakoidc.Name] = p
	providersByType[client.KeyCloakOIDCConfigType] = p
	providersByType[publicclient.KeyCloakOIDCProviderType] = p

	p = genericoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[genericoidc.Name] = true
	providersWithSecrets[genericoidc.Name] = true
	UnrefreshableProviders[genericoidc.Name] = true
	Providers[genericoidc.Name] = p
	providersByType[client.GenericOIDCConfigType] = p
	providersByType[publicclient.GenericOIDCProviderType] = p

	p = cognito.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[cognito.Name] = true
	providersWithSecrets[cognito.Name] = true
	UnrefreshableProviders[cognito.Name] = true
	Providers[cognito.Name] = p
	providersByType[client.CognitoConfigType] = p
	providersByType[publicclient.CognitoProviderType] = p

}

func ProviderLogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	apName := token.GetAuthProvider()
	if apName == "" {
		return nil
	}

	ap, err := GetProvider(apName)
	if err != nil {
		return err
	}
	return ap.LogoutAll(apiContext, token)
}

func ProviderLogout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	apName := token.GetAuthProvider()
	if apName == "" {
		return nil
	}

	ap, err := GetProvider(apName)
	if err != nil {
		return err
	}
	return ap.Logout(apiContext, token)
}

func IsValidUserExtraAttribute(key string) bool {
	if _, ok := userExtraAttributesMap[strings.ToLower(key)]; ok {
		return true
	}
	return false
}

func AuthenticateUser(ctx context.Context, input any, providerName string) (apiv3.Principal, []apiv3.Principal, string, error) {
	return Providers[providerName].AuthenticateUser(ctx, input)
}

func GetPrincipal(principalID string, myToken accessor.TokenAccessor) (apiv3.Principal, error) {
	principal, err := Providers[myToken.GetAuthProvider()].GetPrincipal(principalID, myToken)

	if err != nil && myToken.GetAuthProvider() != LocalProvider {
		p2, e2 := Providers[LocalProvider].GetPrincipal(principalID, myToken)
		if e2 == nil {
			return p2, nil
		}
	}

	return principal, err
}

func SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]apiv3.Principal, error) {
	ap := myToken.GetAuthProvider()
	if ap == "" {
		return []apiv3.Principal{}, fmt.Errorf("[SearchPrincipals] no authProvider specified in token")
	}
	if Providers[ap] == nil {
		return []apiv3.Principal{}, fmt.Errorf("[SearchPrincipals] authProvider %v not initialized", ap)
	}
	principals, err := Providers[ap].SearchPrincipals(name, principalType, myToken)
	if err != nil {
		return principals, err
	}
	if ap != LocalProvider {
		lp := Providers[LocalProvider]
		if lpDedupe, _ := lp.(*local.Provider); lpDedupe != nil {
			localPrincipals, err := lpDedupe.SearchPrincipalsDedupe(name, principalType, myToken, principals)
			if err != nil {
				return principals, err
			}
			principals = append(principals, localPrincipals...)
		}
	}
	return principals, err
}

func CanAccessWithGroupProviders(providerName string, userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	return Providers[providerName].CanAccessWithGroupProviders(userPrincipalID, groups)
}

func RefetchGroupPrincipals(principalID string, providerName string, secret string) ([]apiv3.Principal, error) {
	return Providers[providerName].RefetchGroupPrincipals(principalID, secret)
}

func GetUserExtraAttributes(providerName string, userPrincipal apiv3.Principal) map[string][]string {
	return Providers[providerName].GetUserExtraAttributes(userPrincipal)
}

func IsDisabledProvider(providerName string) (bool, error) {
	provider, err := GetProvider(providerName)
	if err != nil {
		return false, err
	}
	return provider.IsDisabledProvider()
}

// ProviderHasPerUserSecrets returns true if a given provider is known to use per-user auth tokens stored in secrets.
func ProviderHasPerUserSecrets(providerName string) (bool, error) {
	return providersWithSecrets[providerName], nil
}
