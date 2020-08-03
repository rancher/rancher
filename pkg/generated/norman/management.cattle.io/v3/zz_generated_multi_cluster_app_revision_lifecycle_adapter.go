package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type MultiClusterAppRevisionLifecycle interface {
	Create(obj *v3.MultiClusterAppRevision) (runtime.Object, error)
	Remove(obj *v3.MultiClusterAppRevision) (runtime.Object, error)
	Updated(obj *v3.MultiClusterAppRevision) (runtime.Object, error)
}

type multiClusterAppRevisionLifecycleAdapter struct {
	lifecycle MultiClusterAppRevisionLifecycle
}

func (w *multiClusterAppRevisionLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *multiClusterAppRevisionLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *multiClusterAppRevisionLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.MultiClusterAppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *multiClusterAppRevisionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.MultiClusterAppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *multiClusterAppRevisionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.MultiClusterAppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewMultiClusterAppRevisionLifecycleAdapter(name string, clusterScoped bool, client MultiClusterAppRevisionInterface, l MultiClusterAppRevisionLifecycle) MultiClusterAppRevisionHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(MultiClusterAppRevisionGroupVersionResource)
	}
	adapter := &multiClusterAppRevisionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.MultiClusterAppRevision) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
