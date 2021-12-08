package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type KontainerDriverLifecycle interface {
	Create(obj *v3.KontainerDriver) (runtime.Object, error)
	Remove(obj *v3.KontainerDriver) (runtime.Object, error)
	Updated(obj *v3.KontainerDriver) (runtime.Object, error)
}

type kontainerDriverLifecycleAdapter struct {
	lifecycle KontainerDriverLifecycle
}

func (w *kontainerDriverLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *kontainerDriverLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *kontainerDriverLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.KontainerDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *kontainerDriverLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.KontainerDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *kontainerDriverLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.KontainerDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewKontainerDriverLifecycleAdapter(name string, clusterScoped bool, client KontainerDriverInterface, l KontainerDriverLifecycle) KontainerDriverHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(KontainerDriverGroupVersionResource)
	}
	adapter := &kontainerDriverLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.KontainerDriver) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
