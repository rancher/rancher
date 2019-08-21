package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterTemplateRevisionLifecycle interface {
	Create(obj *ClusterTemplateRevision) (runtime.Object, error)
	Remove(obj *ClusterTemplateRevision) (runtime.Object, error)
	Updated(obj *ClusterTemplateRevision) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*ClusterTemplateRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterTemplateRevisionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterTemplateRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterTemplateRevisionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterTemplateRevision))
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
	return func(key string, obj *ClusterTemplateRevision) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
