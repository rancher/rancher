package scim

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/namespace"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	secrets            wcorev1.SecretCache
	isDisabledProvider func(provider string) (bool, error)
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

		provider := mux.Vars(r)["provider"]

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

		labelSet := labels.Set{
			secretKindLabel:   scimAuthToken,
			authProviderLabel: provider,
		}

		list, err := a.secrets.List(tokenSecretNamespace, labelSet.AsSelector())
		if err != nil {
			logrus.Errorf("scim::TokenAuthenticator: failed to list secrets: %s", err)
			writeError(w, NewInternalError())
			return
		}

		authenticated := slices.ContainsFunc(list, func(secret *corev1.Secret) bool {
			return subtle.ConstantTimeCompare([]byte(token), secret.Data["token"]) == 1
		})

		if !authenticated {
			writeError(w, NewError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NewTokenAuthenticator returns a new tokenAuthenticator instance.
func NewTokenAuthenticator(secrets wcorev1.SecretCache) *tokenAuthenticator {
	return &tokenAuthenticator{
		secrets:            secrets,
		isDisabledProvider: providers.IsDisabledProvider,
	}
}
