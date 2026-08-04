package bootstraptoken

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type handler struct {
	secrets corecontrollers.SecretController
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := handler{
		secrets: clients.Core.Secret(),
	}

	h.secrets.OnChange(ctx, "plan-secret", h.OnChange)
}

func (h *handler) OnChange(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Annotations[capr.BootstrapTokenAnnotation] != "true" {
		return secret, nil
	}

	if secret.Annotations[capr.InvalidatedBootstrapTokenAnnotation] == "true" {
		return secret, nil
	}

	if settings.SystemAgentInstallerTokenTTLEnabled.Get() == "false" {
		return secret, nil
	}

	ttl := tokenTTL(secret, time.Now())
	if ttl <= 0 {
		logrus.Debugf("bootstrap token %s/%s expired, invalidating", secret.Namespace, secret.Name)

		secret = secret.DeepCopy()
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}

		secret.Annotations[capr.InvalidatedBootstrapTokenAnnotation] = "true"

		secret, err := h.secrets.Update(secret)
		if err != nil {
			return nil, err
		}

		return secret, nil
	}

	h.secrets.EnqueueAfter(secret.Namespace, secret.Name, ttl)

	return secret, nil
}

func tokenTTL(secret *corev1.Secret, now time.Time) time.Duration {
	shortTTL, err := time.ParseDuration(settings.SystemAgentInstallerTokenShortTTL.Get())
	if err != nil {
		logrus.Warnf("could not parse %s setting: %s", settings.SystemAgentInstallerTokenShortTTL.Name, err)
		return 0
	}

	longTTL, err := time.ParseDuration(settings.SystemAgentInstallerTokenLongTTL.Get())
	if err != nil {
		logrus.Warnf("could not parse %s setting: %s", settings.SystemAgentInstallerTokenLongTTL.Name, err)
		return 0
	}

	finalTTL := secret.CreationTimestamp.Time.Add(longTTL).Sub(now)

	lastAccessString := secret.Annotations[capr.BootstrapTokenLastAccessTimeAnnotation]
	if lastAccessString != "" {
		lastAccessTime, err := time.Parse(time.RFC3339, lastAccessString)
		if err != nil {
			logrus.Warnf("could not parse last access time for %s/%s: %s", secret.Namespace, secret.Name, err)
			return 0
		}

		accessTTL := lastAccessTime.Add(shortTTL).Sub(now)

		if accessTTL < finalTTL {
			finalTTL = accessTTL
		}
	}

	return finalTTL
}
