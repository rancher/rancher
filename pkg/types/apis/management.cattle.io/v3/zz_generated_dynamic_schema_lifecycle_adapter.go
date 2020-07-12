package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type DynamicSchemaLifecycle interface {
	Create(obj *DynamicSchema) (runtime.Object, error)
	Remove(obj *DynamicSchema) (runtime.Object, error)
	Updated(obj *DynamicSchema) (runtime.Object, error)
}

type dynamicSchemaLifecycleAdapter struct {
	lifecycle DynamicSchemaLifecycle
}

func (w *dynamicSchemaLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *dynamicSchemaLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *dynamicSchemaLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*DynamicSchema))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *dynamicSchemaLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*DynamicSchema))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *dynamicSchemaLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*DynamicSchema))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewDynamicSchemaLifecycleAdapter(name string, clusterScoped bool, client DynamicSchemaInterface, l DynamicSchemaLifecycle) DynamicSchemaHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(DynamicSchemaGroupVersionResource)
	}
	adapter := &dynamicSchemaLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *DynamicSchema) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
