package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceQuotaLifecycle interface {
	Create(obj *v1.ResourceQuota) (runtime.Object, error)
	Remove(obj *v1.ResourceQuota) (runtime.Object, error)
	Updated(obj *v1.ResourceQuota) (runtime.Object, error)
}

type resourceQuotaLifecycleAdapter struct {
	lifecycle ResourceQuotaLifecycle
}

func (w *resourceQuotaLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *resourceQuotaLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *resourceQuotaLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ResourceQuota))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *resourceQuotaLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ResourceQuota))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *resourceQuotaLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ResourceQuota))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewResourceQuotaLifecycleAdapter(name string, clusterScoped bool, client ResourceQuotaInterface, l ResourceQuotaLifecycle) ResourceQuotaHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ResourceQuotaGroupVersionResource)
	}
	adapter := &resourceQuotaLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ResourceQuota) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
