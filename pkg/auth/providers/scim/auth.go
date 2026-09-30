package scim

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var tokenSecretNamespace = namespace.GlobalNamespace

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
// kubectl create secret generic scim-okta -n cattle-global-data --from-literal="token=$(sha256 -s $(uuidgen))"
// kubectl label secret -n cattle-global-data scim-okta 'cattle.io/kind=scim-auth-token' 'authn.management.cattle.io/provider=okta'
type tokenAuthenticator struct {
	secretCache        wcorev1.SecretCache
	secrets            wcorev1.SecretClient
	isDisabledProvider func(provider string) (bool, error)
	expireTokensAfter  func() time.Duration
	getConfig          func(provider string) scimconfig.Config
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

		ttl := a.expireTokensAfter()

		var (
			authenticated bool
			tokenID       string
		)
		for _, secret := range list {
			if ttl > 0 && secret.CreationTimestamp.Add(ttl).Before(time.Now()) {
				// Clean up expired tokens, but don't block authentication if deletion fails for some reason
				if err := a.secrets.Delete(tokenSecretNamespace, secret.Name, nil); err != nil {
					logrus.Errorf("scim::TokenAuthenticator: failed to delete expired token secret %s: %s", secret.Name, err)
				}
				continue
			}

			if !authenticated && subtle.ConstantTimeCompare([]byte(token), secret.Data["token"]) == 1 {
				authenticated = true
				tokenID = secret.Name
			}
		}

		if !authenticated {
			writeError(w, NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)))
			return
		}

		setAuditUser(r, provider, tokenID)
		audit.CaptureSCIMRequest(r)

		next.ServeHTTP(w, r)
	})
}

// NewTokenAuthenticator returns a new tokenAuthenticator instance.
func NewTokenAuthenticator(wContext *wrangler.Context) *tokenAuthenticator {
	cmCache := wContext.Core.ConfigMap().Cache()
	return &tokenAuthenticator{
		secretCache:        wContext.Core.Secret().Cache(),
		secrets:            wContext.Core.Secret(),
		isDisabledProvider: providers.IsDisabledProvider,
		expireTokensAfter:  func() time.Duration { return settings.ExpireSCIMTokensAfter.GetDuration() },
		getConfig:          func(provider string) scimconfig.Config { return scimconfig.Get(cmCache, provider) },
	}
}

// setAuditUser records the SCIM caller in the request's audit log entry, if audit logging is enabled.
// tokenID identifies the token that authenticated the request, never its value.
// Group and Extra are replaced, not modified in place, because they share memory with the user info in the request context.
func setAuditUser(r *http.Request, provider, tokenID string) {
	auditUser, ok := audit.FromContext(r.Context())
	if !ok {
		return
	}

	auditUser.Name = "system:scim:" + provider
	auditUser.Group = []string{"system:scim"}
	auditUser.Extra = map[string][]string{
		"scim.cattle.io/provider": {provider},
		"scim.cattle.io/token-id": {tokenID},
	}
}
