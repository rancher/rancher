package scim

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
)

// tokenSecretNamespace is the namespace for SCIM token secrets.
var tokenSecretNamespace = namespace.System

// tokenAuthenticator authenticates requests to SCIM endpoints using Bearer tokens
// stored as secrets in [tokenSecretNamespace].
// Each secret must have the following labels:
//
//	cattle.io/kind: scim-auth-token
//	authn.management.cattle.io/provider: <provider-name>
//
// It's allowed to have multiple tokens per provider to allow token rotation.
//
// Here is an example of how to create a secret with a token for the "okta" provider:
//
// kubectl create secret generic scim-okta -n cattle-system --from-literal="token=$(sha256 -s $(uuidgen))"
// kubectl label secret -n cattle-system scim-okta 'cattle.io/kind=scim-auth-token' 'authn.management.cattle.io/provider=okta'
type tokenAuthenticator struct {
	secretCache        wcorev1.SecretCache
	isDisabledProvider func(configName string) (bool, error)
	expireTokensAfter  func() time.Duration
	getConfig          func(provider string) scimconfig.Config
	authConfigCache    mgmtv3.AuthConfigCache
	getProviderByType  func(authConfigType string) common.AuthProvider
}

// Authenticate implements the http middleware for tokenAuthenticator.
func (a *tokenAuthenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if !(len(parts) == 2 && strings.EqualFold(parts[0], "Bearer")) {
			writeError(w, NewError(http.StatusUnauthorized, "Missing Bearer token"))
			return
		}
		token := parts[1]

		provider := r.PathValue("provider")

		// Check the token before anything else, so a request without a valid token gets the same
		// response whatever the provider's state.
		labelSet := labels.Set{
			secretKindLabel:   scimAuthToken,
			authProviderLabel: provider,
		}

		list, err := a.secretCache.List(tokenSecretNamespace, labelSet.AsSelector())
		if err != nil {
			logrus.Errorf("scim::TokenAuthenticator: failed to list secrets: %s", err)
			writeError(w, NewInternalError())
			return
		}

		var matches []*corev1.Secret
		for _, secret := range list {
			if subtle.ConstantTimeCompare([]byte(token), secret.Data["token"]) == 1 {
				matches = append(matches, secret)
			}
		}

		var (
			authenticated bool
			tokenID       string
		)
		if len(matches) > 0 {
			// Read the setting only after a match, so a request with an invalid token reads nothing
			// but the Secret cache. Expired Secrets are deleted by the token expiry controller.
			ttl, now := a.expireTokensAfter(), time.Now()
			for _, secret := range matches {
				if remaining, expires := tokenExpiresIn(secret, ttl, now); !expires || remaining > 0 {
					authenticated = true
					tokenID = secret.Name
					break
				}
			}
		}

		if !authenticated {
			writeError(w, NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)))
			return
		}

		if provider == local.Name {
			// We don't suppport the "local" provider for SCIM as it's not intended
			// for production use and it doesn't have a group concept.
			writeError(w, NewError(http.StatusNotFound, http.StatusText(http.StatusNotFound)))
			return
		}
		disabled, err := a.isDisabledProvider(provider)

		if err != nil || disabled {
			writeError(w, NewError(http.StatusNotFound, http.StatusText(http.StatusNotFound)))
			return
		}

		cfg := a.getConfig(provider)
		if !cfg.Enabled {
			writeError(w, NewError(http.StatusNotFound, http.StatusText(http.StatusNotFound)))
			return
		}
		if cfg.Paused {
			writeError(w, NewError(http.StatusServiceUnavailable, "SCIM provisioning is temporarily paused"))
			return
		}

		setAuditUser(r, provider, tokenID)
		audit.CaptureSCIMRequest(r)

		next.ServeHTTP(w, r)
	})
}

// NewTokenAuthenticator returns a new tokenAuthenticator instance.
func NewTokenAuthenticator(sContext *config.ScaledContext) *tokenAuthenticator {
	cmCache := sContext.Wrangler.Core.ConfigMap().Cache()
	ta := &tokenAuthenticator{
		secretCache:       sContext.Wrangler.Core.Secret().Cache(),
		authConfigCache:   sContext.Wrangler.Mgmt.AuthConfig().Cache(),
		getProviderByType: providers.GetProviderByType,
		expireTokensAfter: func() time.Duration { return settings.ExpireSCIMTokensAfter.GetDuration() },
		getConfig:         func(provider string) scimconfig.Config { return scimconfig.Get(cmCache, provider) },
	}
	ta.isDisabledProvider = ta.isDisabledProviderFromCache

	return ta
}

// setAuditUser records the SCIM caller in the request's audit log entry, if audit logging is enabled.
// tokenID identifies the token that authenticated the request, never its value.
// Group and Extra are replaced, not modified in place, because they share memory with the user info in the request context.
func setAuditUser(r *http.Request, provider, tokenID string) {
	auditUser, ok := audit.FromContext(r.Context())
	if !ok {
		return
	}

	auditUser.SetIdentity(&user.DefaultInfo{
		Name:   "system:scim:" + provider,
		Groups: []string{"system:scim"},
	})
	// SetIdentity leaves Extra alone, so replace it here.
	auditUser.Extra = map[string][]string{
		"scim.cattle.io/provider": {provider},
		"scim.cattle.io/token-id": {tokenID},
	}
}

// tokenExpiresIn returns how long the token Secret has until it expires after ttl, counted from its
// creation. expires is false when ttl isn't positive, in which case tokens never expire. The token
// is expired when remaining isn't positive.
func tokenExpiresIn(secret *corev1.Secret, ttl time.Duration, now time.Time) (remaining time.Duration, expires bool) {
	if ttl <= 0 {
		return 0, false
	}

	return secret.CreationTimestamp.Add(ttl).Sub(now), true
}

// isDisabledProviderFromCache reports whether the AuthConfig named after the provider in the URL
// is disabled, or has a type no registered provider implements. It reads the generic enabled field,
// which every provider's config embeds, from the cache.
func (a *tokenAuthenticator) isDisabledProviderFromCache(provider string) (bool, error) {
	authConfig, err := a.authConfigCache.Get(provider)
	if err != nil {
		return false, fmt.Errorf("failed to get AuthConfig %s: %w", provider, err)
	}

	return !authConfig.Enabled || a.getProviderByType(authConfig.Type) == nil, nil
}
