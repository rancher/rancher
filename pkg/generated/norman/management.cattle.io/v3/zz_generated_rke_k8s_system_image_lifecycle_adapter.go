package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type RkeK8sSystemImageLifecycle interface {
	Create(obj *v3.RkeK8sSystemImage) (runtime.Object, error)
	Remove(obj *v3.RkeK8sSystemImage) (runtime.Object, error)
	Updated(obj *v3.RkeK8sSystemImage) (runtime.Object, error)
}

type rkeK8sSystemImageLifecycleAdapter struct {
	lifecycle RkeK8sSystemImageLifecycle
}

func (w *rkeK8sSystemImageLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *rkeK8sSystemImageLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *rkeK8sSystemImageLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.RkeK8sSystemImage))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sSystemImageLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.RkeK8sSystemImage))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sSystemImageLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.RkeK8sSystemImage))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRkeK8sSystemImageLifecycleAdapter(name string, clusterScoped bool, client RkeK8sSystemImageInterface, l RkeK8sSystemImageLifecycle) RkeK8sSystemImageHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RkeK8sSystemImageGroupVersionResource)
	}
	adapter := &rkeK8sSystemImageLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.RkeK8sSystemImage) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
