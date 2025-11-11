package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

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
	"github.com/rancher/rancher/pkg/types/config"
)

var (
	ProviderNames          = make(map[string]bool)
	providersWithSecrets   = make(map[string]bool)
	UnrefreshableProviders = make(map[string]bool)
	Providers              = make(map[string]common.AuthProvider)
	LocalProvider          = "local"
	confMu                 sync.Mutex
	userExtraAttributesMap = map[string]bool{common.UserAttributePrincipalID: true, common.UserAttributeUserName: true}
	// Names of all SAML providers. Used to look up the provider based on the type.
	samlProviders = map[string]bool{
		saml.PingName:       true,
		saml.ADFSName:       true,
		saml.KeyCloakName:   true,
		saml.OKTAName:       true,
		saml.ShibbolethName: true,
	}
)

func IsSAMLProviderType(t string) bool {
	return samlProviders[nameFromType(t)]
}

func GetProvider(providerName string) (common.AuthProvider, error) {
	if provider, ok := Providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no such provider '%s'", providerName)
}

func nameFromType(t string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(t), "config"), "provider")
}

func GetProviderByType(t string) common.AuthProvider {
	return Providers[nameFromType(t)]
}

func Configure(ctx context.Context, mgmt *config.ScaledContext) {
	confMu.Lock()
	defer confMu.Unlock()

	userMGR := mgmt.UserManager
	tokenMGR := tokens.NewManager(mgmt.Wrangler)

	var p common.AuthProvider

	p = local.Configure(ctx, mgmt, userMGR)
	ProviderNames[local.Name] = true
	Providers[local.Name] = p

	p = github.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[github.Name] = true
	providersWithSecrets[github.Name] = true
	Providers[github.Name] = p

	p = githubapp.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[githubapp.Name] = true
	providersWithSecrets[githubapp.Name] = true
	Providers[githubapp.Name] = p

	p = azure.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[azure.Name] = true
	providersWithSecrets[azure.Name] = true
	Providers[azure.Name] = p

	p = activedirectory.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[activedirectory.Name] = true
	Providers[activedirectory.Name] = p

	p = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.OpenLdapName)
	ProviderNames[ldap.OpenLdapName] = true
	Providers[ldap.OpenLdapName] = p

	p = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.FreeIpaName)
	ProviderNames[ldap.FreeIpaName] = true
	Providers[ldap.FreeIpaName] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.PingName)
	ProviderNames[saml.PingName] = true
	UnrefreshableProviders[saml.PingName] = true
	Providers[saml.PingName] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.ADFSName)
	ProviderNames[saml.ADFSName] = true
	UnrefreshableProviders[saml.ADFSName] = true
	Providers[saml.ADFSName] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	ProviderNames[saml.KeyCloakName] = true
	UnrefreshableProviders[saml.KeyCloakName] = true
	Providers[saml.KeyCloakName] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.OKTAName)
	ProviderNames[saml.OKTAName] = true
	UnrefreshableProviders[saml.OKTAName] = true
	Providers[saml.OKTAName] = p

	p = saml.Configure(mgmt, userMGR, tokenMGR, saml.ShibbolethName)
	ProviderNames[saml.ShibbolethName] = true
	UnrefreshableProviders[saml.ShibbolethName] = false
	Providers[saml.ShibbolethName] = p

	p = googleoauth.Configure(mgmt, userMGR, tokenMGR)
	ProviderNames[googleoauth.Name] = true
	providersWithSecrets[googleoauth.Name] = true
	Providers[googleoauth.Name] = p

	p = oidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[oidc.Name] = true
	providersWithSecrets[oidc.Name] = true
	Providers[oidc.Name] = p

	p = keycloakoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[keycloakoidc.Name] = true
	providersWithSecrets[keycloakoidc.Name] = true
	Providers[keycloakoidc.Name] = p

	p = genericoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[genericoidc.Name] = true
	providersWithSecrets[genericoidc.Name] = true
	UnrefreshableProviders[genericoidc.Name] = true
	Providers[genericoidc.Name] = p

	p = cognito.Configure(ctx, mgmt, userMGR, tokenMGR)
	ProviderNames[cognito.Name] = true
	providersWithSecrets[cognito.Name] = true
	UnrefreshableProviders[cognito.Name] = true
	Providers[cognito.Name] = p
}

func ProviderLogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	apName := token.GetAuthProvider()
	if apName == "" {
		return nil
	}

	ap, err := GetProvider(apName)
	if err != nil {
		return err
	}
	return ap.LogoutAll(w, r, token)
}

func ProviderLogout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	apName := token.GetAuthProvider()
	if apName == "" {
		return nil
	}

	ap, err := GetProvider(apName)
	if err != nil {
		return err
	}
	return ap.Logout(w, r, token)
}

func IsValidUserExtraAttribute(key string) bool {
	if _, ok := userExtraAttributesMap[strings.ToLower(key)]; ok {
		return true
	}
	return false
}

func AuthenticateUser(w http.ResponseWriter, req *http.Request, input any, providerName string) (apiv3.Principal, []apiv3.Principal, string, error) {
	return Providers[providerName].AuthenticateUser(w, req, input)
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
