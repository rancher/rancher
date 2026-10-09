package providers

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
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
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// mu guards the [providers] map.
	mu sync.RWMutex

	// providers maps provider names to their AuthProvider implementations, populated by Configure.
	providers = make(map[string]common.AuthProvider)

	// authConfigLister lists the configured AuthConfigs, populated by Configure.
	// When nil, each registered provider is assumed to have a single config
	// named after the provider.
	authConfigLister v3.AuthConfigLister

	// lastKnownEnabled caches the most recently confirmed active non-local
	// config. IsExternalProviderEnabled checks this config first, avoiding a
	// full scan when the same config stays enabled across calls — the common
	// production steady state.
	lastKnownEnabled atomic.Value // stores configRef

	// samlProviders lists all SAML provider names. Used to look up the provider based on the type.
	samlProviders = map[string]bool{
		saml.PingName:        true,
		saml.ADFSName:        true,
		saml.KeyCloakName:    true,
		saml.OKTAName:        true,
		saml.ShibbolethName:  true,
		saml.GenericSAMLName: true,
	}
)

// IsSAMLProviderType reports whether the given auth config type belongs to a SAML provider.
func IsSAMLProviderType(t string) bool {
	return samlProviders[NameFromType(t)]
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

// NameFromType converts the type to a providerName.
func NameFromType(t string) string {
	return common.ProviderNameFromType(t)
}

// GetProviderByType returns the registered AuthProvider whose name matches the given auth config type, or nil.
func GetProviderByType(t string) common.AuthProvider {
	mu.RLock()
	defer mu.RUnlock()

	return providers[NameFromType(t)]
}

// Configure initializes all auth providers and registers them in the provider map.
func Configure(ctx context.Context, mgmt *config.ScaledContext) {
	mu.Lock()
	defer mu.Unlock()

	userMGR := mgmt.UserManager
	tokenMGR := tokens.NewManager(mgmt.Wrangler)
	authConfigLister = mgmt.Management.AuthConfigs("").Controller().Lister()

	providers[local.Name] = local.Configure(ctx, mgmt, userMGR)
	providers[github.ProviderName] = github.Configure(mgmt, userMGR, tokenMGR)
	providers[githubapp.ProviderName] = githubapp.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[azure.ProviderName] = azure.Configure(mgmt, userMGR, tokenMGR)
	providers[activedirectory.ProviderName] = activedirectory.Configure(mgmt, userMGR, tokenMGR)
	providers[ldap.OpenLdapName] = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.OpenLdapName)
	providers[ldap.FreeIpaName] = ldap.Configure(mgmt, userMGR, tokenMGR, ldap.FreeIpaName)
	providers[saml.PingName] = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.PingName)
	providers[saml.ADFSName] = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ADFSName)
	providers[saml.KeyCloakName] = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.KeyCloakName)
	providers[saml.OKTAName] = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.OKTAName)
	providers[saml.ShibbolethName] = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.ShibbolethName)
	providers[saml.GenericSAMLName] = saml.Configure(ctx, mgmt, userMGR, tokenMGR, saml.GenericSAMLName)
	providers[googleoauth.ProviderName] = googleoauth.Configure(mgmt, userMGR, tokenMGR)
	providers[oidc.ProviderName] = oidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[keycloakoidc.ProviderName] = keycloakoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[genericoidc.ProviderName] = genericoidc.Configure(ctx, mgmt, userMGR, tokenMGR)
	providers[cognito.ProviderName] = cognito.Configure(ctx, mgmt, userMGR, tokenMGR)
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

// AuthenticateUser delegates authentication to the named provider and returns the resulting principals.
func AuthenticateUser(w http.ResponseWriter, req *http.Request, input any, providerType string) (apiv3.Principal, []apiv3.Principal, string, error) {
	mu.RLock()
	p := providers[NameFromType(providerType)]
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

// SearchPrincipals searches for principals by name using the token's auth
// provider, appending the local results so that users who can log in locally
// remain findable under any provider.
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
		logrus.Debugf("SearchPrincipals failed to search provider %s: %s", ap, err)
		return principals, err
	}

	if ap != local.Name && lp != nil {
		localPrincipals, err := lp.SearchPrincipals(name, principalType, myToken)
		if err != nil {
			return principals, err
		}

		principals = append(principals, localPrincipals...)
	}

	return principals, err
}

// CanAccessWithGroupProviders checks whether the user or any of their group principals have access via the named provider.
func CanAccessWithGroupProviders(providerName string, userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	p, err := GetProvider(providerName)
	if err != nil {
		return false, err
	}

	return p.CanAccessWithGroupProviders(userPrincipalID, groups)
}

// RefetchGroupPrincipals refreshes the group principals for the given user from the named provider.
func RefetchGroupPrincipals(principalID string, providerName string, secret string) ([]apiv3.Principal, error) {
	p, err := GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	return p.RefetchGroupPrincipals(principalID, secret)
}

// GetUserExtraAttributes returns extra attributes for the user principal from the named provider.
func GetUserExtraAttributes(providerType string, userPrincipal apiv3.Principal) map[string][]string {
	mu.RLock()
	p := providers[NameFromType(providerType)]
	mu.RUnlock()
	if p == nil {
		return nil
	}

	return p.GetUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider reports whether this configuration is disbled.
func IsDisabledProvider(providerName, configName string) (bool, error) {
	provider, err := GetProvider(providerName)
	if err != nil {
		return false, err
	}

	return provider.IsDisabledProvider(configName)
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
	lastKnownEnabled.Store(configRef{})
}

// SetAuthConfigLister replaces the AuthConfig lister. Intended for use in tests.
func SetAuthConfigLister(lister v3.AuthConfigLister) {
	mu.Lock()
	defer mu.Unlock()
	authConfigLister = lister
	lastKnownEnabled.Store(configRef{})
}

// configRef identifies an AuthConfig and the provider that implements it.
type configRef struct {
	providerName string
	configName   string
}

// configuredRefs returns the configured AuthConfigs. Without a lister, each
// registered provider is assumed to have a single config named after it.
func configuredRefs() []configRef {
	lister := authConfigLister
	var refs []configRef
	if lister == nil {
		refs = make([]configRef, 0, len(providers))
		for name, p := range providers {
			refs = append(refs, configRef{providerName: name, configName: p.GetName()})
		}

		return refs
	}

	authConfigs, err := lister.List("", labels.Everything())
	if err != nil {
		logrus.Warnf("listing auth configs: %v", err)
		return nil
	}
	refs = make([]configRef, len(authConfigs))
	for i, authConfig := range authConfigs {
		refs[i] = configRef{providerName: NameFromType(authConfig.Type), configName: authConfig.Name}
	}

	return refs
}

// isDisabledConfig reports whether the referenced config is disabled. Its
// IsDisabledProvider implementation makes live Kubernetes API calls, so it
// must not be called with mu held.
func isDisabledConfig(ref configRef) (bool, error) {
	p, err := GetProvider(ref.providerName)
	if err != nil {
		return false, err
	}

	return p.IsDisabledProvider(ref.configName)
}

// IsExternalProviderEnabled reports whether at least one non-local AuthConfig is currently enabled.
// It consults each config's provider IsDisabledProvider rather than the AuthConfig's enabled field.
func IsExternalProviderEnabled() bool {
	// Fast path: check the last known active config first. In the common
	// production steady state a single config stays enabled indefinitely, so
	// one IsDisabledProvider call is enough to confirm and return early — no
	// calls for any other config.
	//
	// alreadyChecked records which config the fast path checked so the full
	// scan below can skip it and avoid a redundant call.
	var alreadyChecked configRef
	if hint, _ := lastKnownEnabled.Load().(configRef); hint != (configRef{}) {
		disabled, err := isDisabledConfig(hint)
		if err == nil {
			if !disabled {
				return true // Hint still valid.
			}
			// Got a clean "disabled" answer — safe to skip in the full scan.
			alreadyChecked = hint
		}
		// On error: alreadyChecked stays empty so the full scan retries this config.
		// Don't clear the hint here; the full scan overwrites it authoritatively.
	}

	// Full scan: snapshot the non-local provider list while holding the lock,
	// then call IsDisabledProvider outside the lock — those implementations make
	// live Kubernetes API calls that must not block the lock.
	mu.RLock()
	for _, ref := range configuredRefs() {
		if ref.providerName == local.Name || ref == alreadyChecked {
			continue
		}
		disabled, err := isDisabledConfig(ref)
		if err != nil {
			logrus.Warnf("checking if auth config %s is disabled: %v", ref.configName, err)
			continue
		}
		if !disabled {
			lastKnownEnabled.Store(ref)
			return true
		}
	}
	mu.RUnlock()

	lastKnownEnabled.Store(configRef{})
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
