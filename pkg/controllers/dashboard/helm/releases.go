package helm

import (
	"context"

	"github.com/rancher/lasso/pkg/client"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogv1 "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/helm"
	"github.com/rancher/wrangler/pkg/apply"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type releaseHandler struct {
	apply               apply.Apply
	sharedClientFactory client.SharedClientFactory
}

func RegisterReleases(ctx context.Context,
	apply apply.Apply,
	shareClientFactory client.SharedClientFactory,
	configMap corecontrollers.ConfigMapController,
	secrets corecontrollers.SecretController,
	releases catalogv1.ReleaseController,
) {
	r := releaseHandler{
		apply: apply.
			WithCacheTypes(releases).
			WithSetID("helm-release").
			WithSetOwnerReference(true, false),
		sharedClientFactory: shareClientFactory,
	}
	configMap.OnChange(ctx, "helm-release", r.OnConfigMapChange)
	secrets.OnChange(ctx, "helm-release", r.OnSecretChange)
}

func (r *releaseHandler) OnConfigMapChange(key string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	spec, err := helm.ToRelease(configMap, r.isNamespaced)
	if err == helm.ErrNotHelmRelease {
		return configMap, nil
	}

	return configMap, r.apply.WithOwner(configMap).ApplyObjects(&v1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.Name,
			Namespace: configMap.Namespace,
		},
		Spec: *spec,
	})
}

func (r *releaseHandler) OnSecretChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	spec, err := helm.ToRelease(secret, r.isNamespaced)
	if err == helm.ErrNotHelmRelease {
		return secret, nil
	}

	return secret, r.apply.WithOwner(secret).ApplyObjects(&v1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Spec: *spec,
	})
}

func (r *releaseHandler) isNamespaced(gvk schema.GroupVersionKind) bool {
	_, nsed, err := r.sharedClientFactory.ResourceForGVK(gvk)
	if err != nil {
		return false
	}
	return nsed
}
