package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type MultiClusterAppLifecycle interface {
	Create(obj *MultiClusterApp) (runtime.Object, error)
	Remove(obj *MultiClusterApp) (runtime.Object, error)
	Updated(obj *MultiClusterApp) (runtime.Object, error)
}

type multiClusterAppLifecycleAdapter struct {
	lifecycle MultiClusterAppLifecycle
}

func (w *multiClusterAppLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *multiClusterAppLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *multiClusterAppLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*MultiClusterApp))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *multiClusterAppLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*MultiClusterApp))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *multiClusterAppLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*MultiClusterApp))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewMultiClusterAppLifecycleAdapter(name string, clusterScoped bool, client MultiClusterAppInterface, l MultiClusterAppLifecycle) MultiClusterAppHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(MultiClusterAppGroupVersionResource)
	}
	adapter := &multiClusterAppLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *MultiClusterApp) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
