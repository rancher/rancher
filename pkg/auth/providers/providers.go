package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/keycloakoidc"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
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

func GetProvider(providerName string) (common.AuthProvider, error) {
	if provider, ok := Providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("No such provider '%s'", providerName)
}

func GetProviderByType(configType string) common.AuthProvider {
	return providersByType[configType]
}

func Configure(ctx context.Context, mgmt *config.ScaledContext) {
	confMu.Lock()
	defer confMu.Unlock()
	userMGR := mgmt.UserManager
	tokenMGR := tokens.NewManager(ctx, mgmt)
	var p common.AuthProvider

	p = local.Configure(ctx, mgmt, tokenMGR)
	ProviderNames[local.Name] = true
	Providers[local.Name] = p
	providersByType[client.LocalConfigType] = p
	providersByType[publicclient.LocalProviderType] = p

	p = github.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[github.Name] = true
	providersWithSecrets[github.Name] = true
	Providers[github.Name] = p
	providersByType[client.GithubConfigType] = p
	providersByType[publicclient.GithubProviderType] = p

	p = azure.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[azure.Name] = true
	providersWithSecrets[azure.Name] = true
	Providers[azure.Name] = p
	providersByType[client.AzureADConfigType] = p
	providersByType[publicclient.AzureADProviderType] = p

	p = activedirectory.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[activedirectory.Name] = true
	Providers[activedirectory.Name] = p
	providersByType[client.ActiveDirectoryConfigType] = p
	providersByType[publicclient.ActiveDirectoryProviderType] = p

	p = ldap.Configure(ctx, mgmt, userMGR, tokenMGR, ldap.OpenLdapName)
	ProviderNames[ldap.OpenLdapName] = true
	Providers[ldap.OpenLdapName] = p
	providersByType[client.OpenLdapConfigType] = p
	providersByType[publicclient.OpenLdapProviderType] = p

	p = ldap.Configure(ctx, mgmt, userMGR, tokenMGR, ldap.FreeIpaName)
	ProviderNames[ldap.FreeIpaName] = true
	Providers[ldap.FreeIpaName] = p
	providersByType[client.FreeIpaConfigType] = p
	providersByType[publicclient.FreeIpaProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.PingName)
	ProviderNames[saml.PingName] = true
	UnrefreshableProviders[saml.PingName] = true
	Providers[saml.PingName] = p
	providersByType[client.PingConfigType] = p
	providersByType[publicclient.PingProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ADFSName)
	ProviderNames[saml.ADFSName] = true
	UnrefreshableProviders[saml.ADFSName] = true
	Providers[saml.ADFSName] = p
	providersByType[client.ADFSConfigType] = p
	providersByType[publicclient.ADFSProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	ProviderNames[saml.KeyCloakName] = true
	UnrefreshableProviders[saml.KeyCloakName] = true
	Providers[saml.KeyCloakName] = p
	providersByType[client.KeyCloakConfigType] = p
	providersByType[publicclient.KeyCloakProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.OKTAName)
	ProviderNames[saml.OKTAName] = true
	UnrefreshableProviders[saml.OKTAName] = true
	Providers[saml.OKTAName] = p
	providersByType[client.OKTAConfigType] = p
	providersByType[publicclient.OKTAProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ShibbolethName)
	ProviderNames[saml.ShibbolethName] = true
	UnrefreshableProviders[saml.ShibbolethName] = false
	Providers[saml.ShibbolethName] = p
	providersByType[client.ShibbolethConfigType] = p
	providersByType[publicclient.ShibbolethProviderType] = p

	p = googleoauth.Configure(ctx, mgmt, userMGR, tokenMGR)
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
}

func IsValidUserExtraAttribute(key string) bool {
	if _, ok := userExtraAttributesMap[strings.ToLower(key)]; ok {
		return true
	}
	return false
}

func AuthenticateUser(ctx context.Context, input interface{}, providerName string) (v3.Principal, []v3.Principal, string, error) {
	return Providers[providerName].AuthenticateUser(ctx, input)
}

func GetPrincipal(principalID string, myToken v3.Token) (v3.Principal, error) {
	principal, err := Providers[myToken.AuthProvider].GetPrincipal(principalID, myToken)

	if err != nil && myToken.AuthProvider != LocalProvider {
		p2, e2 := Providers[LocalProvider].GetPrincipal(principalID, myToken)
		if e2 == nil {
			return p2, nil
		}
	}

	return principal, err
}

func SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	if myToken.AuthProvider == "" {
		return []v3.Principal{}, fmt.Errorf("[SearchPrincipals] no authProvider specified in token")
	}
	if Providers[myToken.AuthProvider] == nil {
		return []v3.Principal{}, fmt.Errorf("[SearchPrincipals] authProvider %v not initialized", myToken.AuthProvider)
	}
	principals, err := Providers[myToken.AuthProvider].SearchPrincipals(name, principalType, myToken)
	if err != nil {
		return principals, err
	}
	if myToken.AuthProvider != LocalProvider {
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

func CanAccessWithGroupProviders(providerName string, userPrincipalID string, groups []v3.Principal) (bool, error) {
	return Providers[providerName].CanAccessWithGroupProviders(userPrincipalID, groups)
}

func RefetchGroupPrincipals(principalID string, providerName string, secret string) ([]v3.Principal, error) {
	return Providers[providerName].RefetchGroupPrincipals(principalID, secret)
}

func GetUserExtraAttributes(providerName string, userPrincipal v3.Principal) map[string][]string {
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
	// For Azure AD, check if it's configured to use the new or old flow. Only the old flow via Azure AD Graph uses per-user secrets.
	if providerName == azure.Name {
		p, ok := Providers[azure.Name]
		if !ok {
			return false, fmt.Errorf("error determining if auth provider uses per-user tokens: provider %s is unknown to Rancher", providerName)
		}

		azureProvider, ok := p.(*azure.Provider)
		if !ok {
			return false, fmt.Errorf("error determining if Azure AD auth provider uses per-user tokens: provider's type is invalid")
		}

		cfg, err := azureProvider.GetAzureConfigK8s()
		if err != nil {
			return false, fmt.Errorf("error determining if Azure AD auth provider uses per-user tokens because of an error to fetch its config: %w", err)
		}

		return azure.IsConfigDeprecated(cfg), nil
	}

	return providersWithSecrets[providerName], nil
}
