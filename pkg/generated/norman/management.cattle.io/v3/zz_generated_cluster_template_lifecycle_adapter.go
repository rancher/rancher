package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterTemplateLifecycle interface {
	Create(obj *v3.ClusterTemplate) (runtime.Object, error)
	Remove(obj *v3.ClusterTemplate) (runtime.Object, error)
	Updated(obj *v3.ClusterTemplate) (runtime.Object, error)
}

type clusterTemplateLifecycleAdapter struct {
	lifecycle ClusterTemplateLifecycle
}

func (w *clusterTemplateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterTemplateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ClusterTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ClusterTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ClusterTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterTemplateLifecycleAdapter(name string, clusterScoped bool, client ClusterTemplateInterface, l ClusterTemplateLifecycle) ClusterTemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterTemplateGroupVersionResource)
	}
	adapter := &clusterTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ClusterTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
