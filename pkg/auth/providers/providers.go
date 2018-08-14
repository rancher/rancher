package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
)

var (
	providers       = make(map[string]common.AuthProvider)
	localProvider   = "local"
	providersByType = make(map[string]common.AuthProvider)
	confMu          sync.Mutex
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

	p = local.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[local.Name] = p
	providersByType[client.LocalConfigType] = p
	providersByType[publicclient.LocalProviderType] = p

	p = github.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[github.Name] = p
	providersByType[client.GithubConfigType] = p
	providersByType[publicclient.GithubProviderType] = p

	p = azure.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[azure.Name] = p
	providersByType[client.AzureADConfigType] = p
	providersByType[publicclient.AzureADProviderType] = p

	p = activedirectory.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[activedirectory.Name] = p
	providersByType[client.ActiveDirectoryConfigType] = p
	providersByType[publicclient.ActiveDirectoryProviderType] = p

	p = ldap.Configure(ctx, mgmt, userMGR, tokenMGR, ldap.OpenLdapName, client.OpenLdapTestAndApplyInputType, "openldap_user", "openldap_group")
	providers[ldap.OpenLdapName] = p
	providersByType[client.OpenLdapConfigType] = p
	providersByType[publicclient.OpenLdapProviderType] = p

	p = ldap.Configure(ctx, mgmt, userMGR, tokenMGR, ldap.FreeIpaName, client.FreeIpaTestAndApplyInputType, "freeipa_user", "freeipa_group")
	providers[ldap.FreeIpaName] = p
	providersByType[client.FreeIpaConfigType] = p
	providersByType[publicclient.FreeIpaProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.PingName)
	providers[saml.PingName] = p
	providersByType[client.PingConfigType] = p
	providersByType[publicclient.PingProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ADFSName)
	providers[saml.ADFSName] = p
	providersByType[client.ADFSConfigType] = p
	providersByType[publicclient.ADFSProviderType] = p

	p = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	providers[saml.KeyCloakName] = p
	providersByType[client.KeyCloakConfigType] = p
	providersByType[publicclient.KeyCloakProviderType] = p
}

func AuthenticateUser(input interface{}, providerName string) (v3.Principal, []v3.Principal, string, error) {
	return providers[providerName].AuthenticateUser(input)
}

func GetPrincipal(principalID string, myToken v3.Token) (v3.Principal, error) {
	principal, err := providers[myToken.AuthProvider].GetPrincipal(principalID, myToken)

	if err != nil && myToken.AuthProvider != localProvider {
		p2, e2 := providers[localProvider].GetPrincipal(principalID, myToken)
		if e2 == nil {
			return p2, nil
		}
	}

	return principal, err
}

func SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	principals, err := providers[myToken.AuthProvider].SearchPrincipals(name, principalType, myToken)
	if err != nil {
		return principals, err
	}
	if myToken.AuthProvider != localProvider {
		lp := providers[localProvider]
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
