package providers

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

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
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

var (
	// mu guards the [providers] map.
	mu sync.RWMutex

	// providers maps provider names to their AuthProvider implementations, populated by Configure.
	providers = make(map[string]common.AuthProvider)

	// lastKnownEnabled caches the name of the most recently confirmed active
	// non-local provider. IsExternalProviderEnabled checks this provider first,
	// avoiding a full scan when the same provider stays enabled across calls —
	// the common production steady state.
	lastKnownEnabled atomic.Value // stores string

	// userExtraAttributesMap defines which token ExtraInfo keys are propagated to UserAttributes.
	userExtraAttributesMap = map[string]bool{common.UserAttributePrincipalID: true, common.UserAttributeUserName: true}

	// samlProviders lists all SAML provider names. Used to look up the provider based on the type.
	samlProviders = map[string]bool{
		saml.PingName:       true,
		saml.ADFSName:       true,
		saml.KeyCloakName:   true,
		saml.OKTAName:       true,
		saml.ShibbolethName: true,
	}
)

// IsSAMLProviderType reports whether the given auth config type belongs to a SAML provider.
func IsSAMLProviderType(t string) bool {
	return samlProviders[nameFromType(t)]
}

// GetProvider returns the registered AuthProvider for the given name or an error if not found.
func GetProvider(providerName string) (common.AuthProvider, error) {
	mu.RLock()
	provider, ok := providers[providerName]
	mu.RUnlock()

	if ok && provider != nil {
		return provider, nil
	}

	return nil, fmt.Errorf("no such provider '%s'", providerName)
}

func nameFromType(t string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(t), "config"), "provider")
}

// GetProviderByType returns the registered AuthProvider whose name matches the given auth config type, or nil.
func GetProviderByType(t string) common.AuthProvider {
	mu.RLock()
	defer mu.RUnlock()

	return providers[nameFromType(t)]
}

// Configure initializes all auth providers and registers them in the provider map.
func Configure(ctx context.Context, mgmt *config.ScaledContext) {
	mu.Lock()
	defer mu.Unlock()

	userMGR := mgmt.UserManager
	tokenMGR := tokens.NewManager(mgmt.Wrangler)

	providers[local.Name] = local.Configure(ctx, mgmt, userMGR)
	providers[github.Name] = github.Configure(mgmt, userMGR, tokenMGR)
	providers[githubapp.Name] = githubapp.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[azure.Name] = azure.Configure(mgmt, userMGR, tokenMGR)
	providers[activedirectory.Name] = activedirectory.Configure(mgmt, userMGR, tokenMGR)
	providers[ldap.OpenLdapName] = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.OpenLdapName)
	providers[ldap.FreeIpaName] = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.FreeIpaName)
	providers[saml.PingName] = saml.Configure(mgmt, userMGR, tokenMGR, saml.PingName)
	providers[saml.ADFSName] = saml.Configure(mgmt, userMGR, tokenMGR, saml.ADFSName)
	providers[saml.KeyCloakName] = saml.Configure(mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	providers[saml.OKTAName] = saml.Configure(mgmt, userMGR, tokenMGR, saml.OKTAName)
	providers[saml.ShibbolethName] = saml.Configure(mgmt, userMGR, tokenMGR, saml.ShibbolethName)
	providers[googleoauth.Name] = googleoauth.Configure(mgmt, userMGR, tokenMGR)
	providers[oidc.Name] = oidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[keycloakoidc.Name] = keycloakoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[genericoidc.Name] = genericoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[cognito.Name] = cognito.Configure(ctx, mgmt, userMGR, tokenMGR)
}

// ProviderLogoutAll logs out the user from all sessions for the token's auth provider.
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

// ProviderLogout logs out the current session for the token's auth provider.
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

// IsValidUserExtraAttribute reports whether key is a recognized extra attribute for user propagation.
func IsValidUserExtraAttribute(key string) bool {
	if _, ok := userExtraAttributesMap[strings.ToLower(key)]; ok {
		return true
	}

	return false
}

// AuthenticateUser delegates authentication to the named provider and returns the resulting principals.
func AuthenticateUser(w http.ResponseWriter, req *http.Request, input any, providerName string) (apiv3.Principal, []apiv3.Principal, string, error) {
	mu.RLock()
	p := providers[providerName]
	mu.RUnlock()

	return p.AuthenticateUser(w, req, input)
}

// GetPrincipal looks up a principal by ID using the token's auth provider, falling back to the local provider.
func GetPrincipal(principalID string, myToken accessor.TokenAccessor) (apiv3.Principal, error) {
	mu.RLock()
	p := providers[myToken.GetAuthProvider()]
	lp := providers[local.Name]
	mu.RUnlock()

	principal, err := p.GetPrincipal(principalID, myToken)
	if err != nil && myToken.GetAuthProvider() != local.Name {
		p2, e2 := lp.GetPrincipal(principalID, myToken)
		if e2 == nil {
			return p2, nil
		}
	}

	return principal, err
}

// SearchPrincipals searches for principals by name using the token's auth provider, appending deduplicated local results.
func SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]apiv3.Principal, error) {
	ap := myToken.GetAuthProvider()
	if ap == "" {
		return []apiv3.Principal{}, fmt.Errorf("[SearchPrincipals] no authProvider specified in token")
	}

	mu.RLock()
	p := providers[ap]
	lp := providers[local.Name]
	mu.RUnlock()

	if p == nil {
		return []apiv3.Principal{}, fmt.Errorf("[SearchPrincipals] authProvider %v not initialized", ap)
	}
	principals, err := p.SearchPrincipals(name, principalType, myToken)
	if err != nil {
		return principals, err
	}
	if ap != local.Name {
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

// CanAccessWithGroupProviders checks whether the user or any of their group principals have access via the named provider.
func CanAccessWithGroupProviders(providerName string, userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	mu.RLock()
	p := providers[providerName]
	mu.RUnlock()

	return p.CanAccessWithGroupProviders(userPrincipalID, groups)
}

// RefetchGroupPrincipals refreshes the group principals for the given user from the named provider.
func RefetchGroupPrincipals(principalID string, providerName string, secret string) ([]apiv3.Principal, error) {
	mu.RLock()
	p := providers[providerName]
	mu.RUnlock()

	return p.RefetchGroupPrincipals(principalID, secret)
}

// GetUserExtraAttributes returns extra attributes for the user principal from the named provider.
func GetUserExtraAttributes(providerName string, userPrincipal apiv3.Principal) map[string][]string {
	mu.RLock()
	p := providers[providerName]
	mu.RUnlock()

	return p.GetUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider reports whether the named provider is currently disabled.
func IsDisabledProvider(providerName string) (bool, error) {
	provider, err := GetProvider(providerName)
	if err != nil {
		return false, err
	}

	return provider.IsDisabledProvider()
}

// ProviderNames returns the names of all registered providers.
func ProviderNames() []string {
	mu.RLock()
	defer mu.RUnlock()

	return slices.Collect(maps.Keys(providers))
}

// SetProviders replaces the provider map. Intended for use in tests.
func SetProviders(m map[string]common.AuthProvider) {
	mu.Lock()
	defer mu.Unlock()
	if m == nil {
		m = make(map[string]common.AuthProvider)
	}
	providers = m
	lastKnownEnabled.Store("")
}

// IsExternalProviderEnabled reports whether at least one non-local auth provider is currently enabled.
// It consults each registered provider's IsDisabledProvider rather than querying auth config resources.
func IsExternalProviderEnabled() bool {
	// Fast path: check the last known active provider first. In the common
	// production steady state a single provider stays enabled indefinitely, so
	// one IsDisabledProvider call is enough to confirm and return early — no
	// snapshot allocation, no calls to any other provider.
	//
	// alreadyChecked records which provider the fast path called IsDisabledProvider
	// on so the full scan below can skip it and avoid a redundant call.
	var alreadyChecked string
	if hint, _ := lastKnownEnabled.Load().(string); hint != "" {
		mu.RLock()
		p := providers[hint]
		mu.RUnlock()
		if p != nil {
			alreadyChecked = hint
			if disabled, err := p.IsDisabledProvider(); err == nil && !disabled {
				return true
			}
		}
		lastKnownEnabled.Store("") // Stale; clear before full scan.
	}

	// Full scan: snapshot the non-local provider list while holding the lock,
	// then call IsDisabledProvider outside the lock — those implementations make
	// live Kubernetes API calls that must not block the lock.
	mu.RLock()
	type entry struct {
		name     string
		provider common.AuthProvider
	}
	snapshot := make([]entry, 0, len(providers))
	for name, p := range providers {
		if name != local.Name && name != alreadyChecked {
			snapshot = append(snapshot, entry{name, p})
		}
	}
	mu.RUnlock()

	for _, e := range snapshot {
		disabled, err := e.provider.IsDisabledProvider()
		if err != nil {
			logrus.Warnf("checking if provider %s is disabled: %v", e.name, err)
			continue
		}
		if !disabled {
			lastKnownEnabled.Store(e.name)
			return true
		}
	}
	return false
}

// IsLocalHidden reports whether the local auth provider should be hidden from public-facing endpoints.
// It returns true when the HideLocalAuthProvider feature flag is enabled and at least one external
// auth provider is currently active.
func IsLocalHidden() bool {
	return features.HideLocalAuthProvider.Enabled() && IsExternalProviderEnabled()
}

// ProviderUsesUserSecrets reports whether the named provider stores per-user secrets for token refresh.
func ProviderUsesUserSecrets(providerName string) bool {
	mu.RLock()
	p, ok := providers[providerName]
	mu.RUnlock()
	if ok {
		return p.UsesUserSecrets()
	}

	return false
}

// ProviderCanRefreshPrincipals reports whether the named provider supports refreshing group principals.
func ProviderCanRefreshPrincipals(providerName string) bool {
	mu.RLock()
	p, ok := providers[providerName]
	mu.RUnlock()
	if ok {
		return p.CanRefreshPrincipals()
	}

	return false
}
