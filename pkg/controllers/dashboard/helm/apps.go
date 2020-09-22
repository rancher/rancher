package helm

import (
	"context"

	"github.com/docker/docker/pkg/locker"
	"github.com/rancher/lasso/pkg/client"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/helm"
	catalogv1 "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/wrangler/pkg/apply"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type appHandler struct {
	apply               apply.Apply
	sharedClientFactory client.SharedClientFactory
	configMapCache      corecontrollers.ConfigMapCache
	secretCache         corecontrollers.SecretCache
	locker              locker.Locker
}

func RegisterApps(ctx context.Context,
	apply apply.Apply,
	shareClientFactory client.SharedClientFactory,
	configMap corecontrollers.ConfigMapController,
	secrets corecontrollers.SecretController,
	apps catalogv1.AppController,
) {
	r := appHandler{
		apply: apply.
			WithCacheTypes(apps).
			WithSetID("helm-app").
			WithSetOwnerReference(true, false),
		sharedClientFactory: shareClientFactory,
		secretCache:         secrets.Cache(),
		configMapCache:      configMap.Cache(),
	}
	configMap.OnChange(ctx, "helm-app", r.OnConfigMapChange)
	secrets.OnChange(ctx, "helm-app", r.OnSecretChange)
	catalogv1.RegisterAppStatusHandler(ctx, apps, "", "helm-app-status", r.appStatus)
	relatedresource.Watch(ctx, "helm-app",
		relatedresource.OwnerResolver(true, "v1", "ConfigMap"),
		configMap,
		apps)
	relatedresource.Watch(ctx, "helm-app",
		relatedresource.OwnerResolver(true, "v1", "Secrets"),
		secrets,
		apps)
}

func (a *appHandler) appStatus(app *v1.App, status v1.ReleaseStatus) (v1.ReleaseStatus, error) {
	summary := v1.Summary{}
	if app.Spec.Info != nil && app.Spec.Info.Status != v1.StatusUnknown {
		summary.State = string(app.Spec.Info.Status)
		switch app.Spec.Info.Status {
		case v1.StatusDeployed:
		case v1.StatusUninstalled:
		case v1.StatusSuperseded:
		case v1.StatusFailed:
			summary.Error = true
		case v1.StatusUninstalling:
			summary.Transitioning = true
		case v1.StatusPendingInstall:
			summary.Transitioning = true
		case v1.StatusPendingUpgrade:
			summary.Transitioning = true
		case v1.StatusPendingRollback:
			summary.Transitioning = true
		}
	}

	status.Summary = summary
	status.ObservedGeneration = app.Generation

	return status, nil
}

func (a *appHandler) isLatestSecret(spec *v1.ReleaseSpec) (bool, error) {
	others, err := a.secretCache.List(spec.Namespace, labels.SelectorFromSet(labels.Set{
		"owner": "helm",
	}))
	if err != nil {
		return false, err
	}

	othersRuntime := make([]runtime.Object, 0, len(others))
	for _, other := range others {
		othersRuntime = append(othersRuntime, other)
	}

	return helm.IsLatest(spec, othersRuntime), nil
}

func (a *appHandler) isLatestConfigMap(spec *v1.ReleaseSpec) (bool, error) {
	others, err := a.configMapCache.List(spec.Namespace, labels.SelectorFromSet(labels.Set{
		"OWNER": "TILLER",
	}))
	if err != nil {
		return false, err
	}

	othersRuntime := make([]runtime.Object, 0, len(others))
	for _, other := range others {
		othersRuntime = append(othersRuntime, other)
	}

	return helm.IsLatest(spec, othersRuntime), nil
}

func (a *appHandler) OnConfigMapChange(key string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	spec, err := helm.ToRelease(configMap, a.isNamespaced)
	if err == helm.ErrNotHelmRelease {
		return configMap, nil
	}

	a.locker.Lock(spec.Name)
	defer a.locker.Unlock(spec.Name)

	if latest, err := a.isLatestConfigMap(spec); err != nil {
		return nil, err
	} else if !latest {
		// Don't delete if we create an App before as it's probably owned by something else now
		return nil, generic.ErrSkip
	}

	return configMap, a.apply.WithOwner(configMap).ApplyObjects(&v1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: configMap.Namespace,
		},
		Spec: *spec,
	})
}

func (a *appHandler) OnSecretChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	spec, err := helm.ToRelease(secret, a.isNamespaced)
	if err == helm.ErrNotHelmRelease {
		return secret, nil
	}

	a.locker.Lock(spec.Name)
	defer a.locker.Unlock(spec.Name)

	if latest, err := a.isLatestSecret(spec); err != nil {
		return nil, err
	} else if !latest {
		// Don't delete if we create an App before as it's probably owned by something else now
		return nil, generic.ErrSkip
	}

	return secret, a.apply.WithOwner(secret).ApplyObjects(&v1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: secret.Namespace,
		},
		Spec: *spec,
	})
}

func (a *appHandler) isNamespaced(gvk schema.GroupVersionKind) bool {
	_, nsed, err := a.sharedClientFactory.ResourceForGVK(gvk)
	if err != nil {
		return false
	}
	return nsed
}
