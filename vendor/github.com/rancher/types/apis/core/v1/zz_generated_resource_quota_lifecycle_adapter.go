package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceQuotaLifecycle interface {
	Create(obj *v1.ResourceQuota) (*v1.ResourceQuota, error)
	Remove(obj *v1.ResourceQuota) (*v1.ResourceQuota, error)
	Updated(obj *v1.ResourceQuota) (*v1.ResourceQuota, error)
}

type resourceQuotaLifecycleAdapter struct {
	lifecycle ResourceQuotaLifecycle
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
	adapter := &resourceQuotaLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ResourceQuota) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
