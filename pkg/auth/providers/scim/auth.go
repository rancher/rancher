package scim

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
	isDisabledProvider func(configName string) (bool, error)
	expireTokensAfter  func() time.Duration
	getConfig          func(provider string) scimconfig.Config
	authConfigs        v3.AuthConfigInterface
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
func NewTokenAuthenticator(sContext *config.ScaledContext) *tokenAuthenticator {
	cmCache := sContext.Wrangler.Core.ConfigMap().Cache()
	ta := &tokenAuthenticator{
		secretCache:       sContext.Wrangler.Core.Secret().Cache(),
		secrets:           sContext.Wrangler.Core.Secret(),
		authConfigs:       sContext.Management.AuthConfigs(""),
		expireTokensAfter: func() time.Duration { return settings.ExpireSCIMTokensAfter.GetDuration() },
		getConfig:         func(provider string) scimconfig.Config { return scimconfig.Get(cmCache, provider) },
	}
	ta.isDisabledProvider = ta.isDisabledProviderFromResource

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

	auditUser.Name = "system:scim:" + provider
	auditUser.Group = []string{"system:scim"}
	auditUser.Extra = map[string][]string{
		"scim.cattle.io/provider": {provider},
		"scim.cattle.io/token-id": {tokenID},
	}
}

func (a *tokenAuthenticator) isDisabledProviderFromResource(provider string) (bool, error) {
	// This gets the AuthConfig by name (which comes from the URL).
	authConfigObj, err := a.authConfigs.ObjectClient().UnstructuredClient().Get(provider, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to retrieve AuthConfig %s: %w", provider, err)
	}
	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return false, fmt.Errorf("failed to parse AuthConfig %s: %w", provider, err)
	}

	// We could use .enabled from the unstructured content but instead this
	// converts the type to a name to delegate the IsDisabledProvider check to
	// the actual provider.
	rawType, ok := u.UnstructuredContent()["type"].(string)
	if !ok {
		return false, fmt.Errorf("invalid AuthConfig %s missing type", provider)
	}

	providerName := providers.NameFromType(rawType)

	return providers.IsDisabledProvider(providerName, provider)
}
