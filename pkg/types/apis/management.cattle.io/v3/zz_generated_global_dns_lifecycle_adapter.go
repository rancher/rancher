package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlobalDnsLifecycle interface {
	Create(obj *GlobalDns) (runtime.Object, error)
	Remove(obj *GlobalDns) (runtime.Object, error)
	Updated(obj *GlobalDns) (runtime.Object, error)
}

type globalDnsLifecycleAdapter struct {
	lifecycle GlobalDnsLifecycle
}

func (w *globalDnsLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *globalDnsLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *globalDnsLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GlobalDns))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalDnsLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GlobalDns))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalDnsLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GlobalDns))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGlobalDnsLifecycleAdapter(name string, clusterScoped bool, client GlobalDnsInterface, l GlobalDnsLifecycle) GlobalDnsHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(GlobalDnsGroupVersionResource)
	}
	adapter := &globalDnsLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *GlobalDns) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
