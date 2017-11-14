package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type DynamicSchemaLifecycle interface {
	Create(obj *DynamicSchema) error
	Remove(obj *DynamicSchema) error
	Updated(obj *DynamicSchema) error
}

type dynamicSchemaLifecycleAdapter struct {
	lifecycle DynamicSchemaLifecycle
}

func (w *dynamicSchemaLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*DynamicSchema))
}

func (w *dynamicSchemaLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*DynamicSchema))
}

func (w *dynamicSchemaLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*DynamicSchema))
}

func NewDynamicSchemaLifecycleAdapter(name string, client DynamicSchemaInterface, l DynamicSchemaLifecycle) DynamicSchemaHandlerFunc {
	adapter := &dynamicSchemaLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *DynamicSchema) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
