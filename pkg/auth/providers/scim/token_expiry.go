package scim

import (
	"context"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// tokenExpiryHandler deletes SCIM token Secrets when they expire, as set by
// [settings.ExpireSCIMTokensAfter].
type tokenExpiryHandler struct {
	secrets           wcorev1.SecretController
	secretCache       wcorev1.SecretCache
	expireTokensAfter func() time.Duration
}

// RegisterTokenExpiry registers the handlers that delete SCIM token Secrets when they expire.
//
// The Secret handler is added to the shared Secret controller, which watches Secrets in all
// namespaces, so it runs for every Secret change and ignores all but token Secrets in
// [tokenSecretNamespace]. This is on purpose. Rancher already watches and caches every Secret in
// the local cluster through this controller, and the SCIM authenticator lists token Secrets from
// the same cache. The handler adds no watch, cache or queue, only a few field checks per Secret
// event.
//
// A controller limited to one namespace would need its own cache factory, as the impersonation and
// cluster auth token controllers use on downstream clusters to avoid caching every Secret there.
// Here it would add a second watch and a second cached copy of every Secret in
// [tokenSecretNamespace], including large Helm release Secrets, and would need its own start and
// cache sync.
func RegisterTokenExpiry(ctx context.Context, secrets wcorev1.SecretController, settingController mgmtv3.SettingController) {
	h := &tokenExpiryHandler{
		secrets:           secrets,
		secretCache:       secrets.Cache(),
		expireTokensAfter: func() time.Duration { return settings.ExpireSCIMTokensAfter.GetDuration() },
	}

	secrets.OnChange(ctx, "scim-token-expiry", h.onSecretChange)
	settingController.OnChange(ctx, "scim-token-expiry-setting", h.onSettingChange)
}

// onSecretChange deletes an expired token Secret, or requeues it for the time it expires.
func (h *tokenExpiryHandler) onSecretChange(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.DeletionTimestamp != nil ||
		secret.Namespace != tokenSecretNamespace || secret.Labels[secretKindLabel] != scimAuthToken {
		return secret, nil
	}

	remaining, expires := tokenExpiresIn(secret, h.expireTokensAfter(), time.Now())
	if !expires {
		return secret, nil
	}
	if remaining > 0 {
		h.secrets.EnqueueAfter(secret.Namespace, secret.Name, remaining)
		return secret, nil
	}

	// Use the constant, not the Secret's namespace, so a mistake in the checks above can't delete a
	// Secret in another namespace.
	err := h.secrets.Delete(tokenSecretNamespace, secret.Name, &metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &secret.UID},
	})
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return secret, nil // Already deleted, or replaced by a new Secret with the same name.
	}
	if err != nil {
		return secret, fmt.Errorf("failed to delete expired SCIM token secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	logrus.Infof("Deleted expired SCIM token secret %s/%s for provider %s", secret.Namespace, secret.Name, secret.Labels[authProviderLabel])

	return secret, nil
}

// onSettingChange requeues every token Secret when the expiry setting changes, so each one is
// deleted or requeued for its new expiry time.
func (h *tokenExpiryHandler) onSettingChange(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || setting.Name != settings.ExpireSCIMTokensAfter.Name {
		return setting, nil
	}

	list, err := h.secretCache.List(tokenSecretNamespace, labels.Set{secretKindLabel: scimAuthToken}.AsSelector())
	if err != nil {
		return setting, fmt.Errorf("failed to list SCIM token secrets: %w", err)
	}

	for _, secret := range list {
		h.secrets.Enqueue(secret.Namespace, secret.Name)
	}

	return setting, nil
}
