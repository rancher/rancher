package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
)

var (
	ProviderNames          = make(map[string]bool)
	ProvidersWithSecrets   = make(map[string]bool)
	UnrefreshableProviders = make(map[string]bool)
	providers              = make(map[string]common.AuthProvider)
	LocalProvider          = "local"
	providersByType        = make(map[string]common.AuthProvider)
	confMu                 sync.Mutex
)

func GetProvider(providerName string) (common.AuthProvider, error) {
	if provider, ok := providers[providerName]; ok {
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
	providers[local.Name] = p
	providersByType[client.LocalConfigType] = p
	providersByType[publicclient.LocalProviderType] = p

	p = github.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[github.Name] = true
	ProvidersWithSecrets[github.Name] = true
	providers[github.Name] = p
	providersByType[client.GithubConfigType] = p
	providersByType[publicclient.GithubProviderType] = p

	p = azure.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[azure.Name] = true
	ProvidersWithSecrets[azure.Name] = true
	providers[azure.Name] = p
	providersByType[client.AzureADConfigType] = p
	providersByType[publicclient.AzureADProviderType] = p

	p = activedirectory.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[activedirectory.Name] = true
	providers[activedirectory.Name] = p
	providersByType[client.ActiveDirectoryConfigType] = p
	providersByType[publicclient.ActiveDirectoryProviderType] = p

	remoteConfig := ldap.NewRemoteConfig(mgmt.Management.AuthConfigs(""))
	// TODO Is there a config source where it would be sensible to put expireAfter?
	expireAfter := time.Minute * 5
	cachedConfig := NewCachedConfig(remoteConfig, expireAfter)
	p = ldap.Configure(ctx, mgmt, userMGR, tokenMGR, ldap.OpenLdapName, cachedConfig)
	ProviderNames[ldap.OpenLdapName] = true
	providers[ldap.OpenLdapName] = p
	providersByType[client.OpenLdapConfigType] = p
	providersByType[publicclient.OpenLdapProviderType] = p

	p = ldap.Configure(ctx, mgmt, userMGR, tokenMGR, ldap.FreeIpaName, cachedConfig)
	ProviderNames[ldap.FreeIpaName] = true
	providers[ldap.FreeIpaName] = p
	providersByType[client.FreeIpaConfigType] = p
	providersByType[publicclient.FreeIpaProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.PingName)
	ProviderNames[saml.PingName] = true
	UnrefreshableProviders[saml.PingName] = true
	providers[saml.PingName] = p
	providersByType[client.PingConfigType] = p
	providersByType[publicclient.PingProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ADFSName)
	ProviderNames[saml.ADFSName] = true
	UnrefreshableProviders[saml.ADFSName] = true
	providers[saml.ADFSName] = p
	providersByType[client.ADFSConfigType] = p
	providersByType[publicclient.ADFSProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	ProviderNames[saml.KeyCloakName] = true
	UnrefreshableProviders[saml.KeyCloakName] = true
	providers[saml.KeyCloakName] = p
	providersByType[client.KeyCloakConfigType] = p
	providersByType[publicclient.KeyCloakProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.OKTAName)
	ProviderNames[saml.OKTAName] = true
	UnrefreshableProviders[saml.OKTAName] = true
	providers[saml.OKTAName] = p
	providersByType[client.OKTAConfigType] = p
	providersByType[publicclient.OKTAProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ShibbolethName)
	ProviderNames[saml.ShibbolethName] = true
	UnrefreshableProviders[saml.ShibbolethName] = false
	providers[saml.ShibbolethName] = p
	providersByType[client.ShibbolethConfigType] = p
	providersByType[publicclient.ShibbolethProviderType] = p

	p = googleoauth.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[googleoauth.Name] = true
	ProvidersWithSecrets[googleoauth.Name] = true
	providers[googleoauth.Name] = p
	providersByType[client.GoogleOauthConfigType] = p
	providersByType[publicclient.GoogleOAuthProviderType] = p
}

func AuthenticateUser(ctx context.Context, input interface{}, providerName string) (v3.Principal, []v3.Principal, string, error) {
	return providers[providerName].AuthenticateUser(ctx, input)
}

func GetPrincipal(principalID string, myToken v3.Token) (v3.Principal, error) {
	principal, err := providers[myToken.AuthProvider].GetPrincipal(principalID, myToken)

	if err != nil && myToken.AuthProvider != LocalProvider {
		p2, e2 := providers[LocalProvider].GetPrincipal(principalID, myToken)
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
	if providers[myToken.AuthProvider] == nil {
		return []v3.Principal{}, fmt.Errorf("[SearchPrincipals] authProvider %v not initialized", myToken.AuthProvider)
	}
	principals, err := providers[myToken.AuthProvider].SearchPrincipals(name, principalType, myToken)
	if err != nil {
		return principals, err
	}
	if myToken.AuthProvider != LocalProvider {
		lp := providers[LocalProvider]
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
	return providers[providerName].CanAccessWithGroupProviders(userPrincipalID, groups)
}

func RefetchGroupPrincipals(principalID string, providerName string, secret string) ([]v3.Principal, error) {
	return providers[providerName].RefetchGroupPrincipals(principalID, secret)
}
