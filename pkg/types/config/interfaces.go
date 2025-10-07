package config

import (
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbac "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type nonNamespacedCacheAndClient[T generic.RuntimeMetaObject, TList runtime.Object] interface {
	generic.NonNamespacedClientInterface[T, TList]
	Cache() generic.NonNamespacedCacheInterface[T]
	Informer() cache.SharedIndexInformer // needed to immediately start the caches
}

type namespacedCacheAndClient[T generic.RuntimeMetaObject, TList runtime.Object] interface {
	generic.ClientInterface[T, TList]
	Cache() generic.CacheInterface[T]
	Informer() cache.SharedIndexInformer // needed to immediately start the caches
}

// rbacInterface does not restrict any usage of the original interface, as Steve's accesscontrol uses caches and indexers for all resources
type rbacInterface wrbac.Interface

type coreInterface struct {
	factory *corecontrollers.Factory
}

func (i coreInterface) Namespace() wcore.NamespaceController {
	return i.factory.Core().V1().Namespace()
}

func (i coreInterface) ConfigMap() namespacedCacheAndClient[*corev1.ConfigMap, *corev1.ConfigMapList] {
	return i.factory.Core().V1().ConfigMap()
}

func (i coreInterface) Secret() namespacedCacheAndClient[*corev1.Secret, *corev1.SecretList] {
	return i.factory.Core().V1().Secret()
}

// ComponentStatus is deprecated since k8s 1.19 and wrangler's generated clients do not support it, but it seems it may still be implemented/used
func (i coreInterface) ComponentStatus() generic.NonNamespacedClientInterface[*corev1.ComponentStatus, *corev1.ComponentStatusList] {
	return generic.NewNonNamespacedController[*corev1.ComponentStatus, *corev1.ComponentStatusList](schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ComponentStatus"}, "componentstatuses", i.factory.ControllerFactory())
}

func (i coreInterface) Node() wcore.NodeController {
	return i.factory.Core().V1().Node()
}

func (i coreInterface) Pod() wcore.PodController {
	return i.factory.Core().V1().Pod()
}

func (i coreInterface) Service() wcore.ServiceController {
	return i.factory.Core().V1().Service()
}

func (i coreInterface) ServiceAccount() namespacedCacheAndClient[*corev1.ServiceAccount, *corev1.ServiceAccountList] {
	return i.factory.Core().V1().ServiceAccount()
}

func (i coreInterface) LimitRange() wcore.LimitRangeController {
	return i.factory.Core().V1().LimitRange()
}

func (i coreInterface) ResourceQuota() wcore.ResourceQuotaController {
	return i.factory.Core().V1().ResourceQuota()
}
