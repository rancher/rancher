package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceQuotaTemplateLifecycle interface {
	Create(obj *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error)
	Remove(obj *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error)
	Updated(obj *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error)
}

type resourceQuotaTemplateLifecycleAdapter struct {
	lifecycle ResourceQuotaTemplateLifecycle
}

func (w *resourceQuotaTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ResourceQuotaTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *resourceQuotaTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ResourceQuotaTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *resourceQuotaTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ResourceQuotaTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewResourceQuotaTemplateLifecycleAdapter(name string, clusterScoped bool, client ResourceQuotaTemplateInterface, l ResourceQuotaTemplateLifecycle) ResourceQuotaTemplateHandlerFunc {
	adapter := &resourceQuotaTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ResourceQuotaTemplate) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
