package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterTemplateRevisionLifecycle interface {
	Create(obj *v3.ClusterTemplateRevision) (runtime.Object, error)
	Remove(obj *v3.ClusterTemplateRevision) (runtime.Object, error)
	Updated(obj *v3.ClusterTemplateRevision) (runtime.Object, error)
}

type clusterTemplateRevisionLifecycleAdapter struct {
	lifecycle ClusterTemplateRevisionLifecycle
}

func (w *clusterTemplateRevisionLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterTemplateRevisionLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterTemplateRevisionLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ClusterTemplateRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterTemplateRevisionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ClusterTemplateRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterTemplateRevisionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ClusterTemplateRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterTemplateRevisionLifecycleAdapter(name string, clusterScoped bool, client ClusterTemplateRevisionInterface, l ClusterTemplateRevisionLifecycle) ClusterTemplateRevisionHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterTemplateRevisionGroupVersionResource)
	}
	adapter := &clusterTemplateRevisionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ClusterTemplateRevision) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
