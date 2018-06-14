package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type FreeIpaTestAndApplyInputLifecycle interface {
	Create(obj *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error)
	Remove(obj *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error)
	Updated(obj *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error)
}

type freeIpaTestAndApplyInputLifecycleAdapter struct {
	lifecycle FreeIpaTestAndApplyInputLifecycle
}

func (w *freeIpaTestAndApplyInputLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*FreeIpaTestAndApplyInput))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *freeIpaTestAndApplyInputLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*FreeIpaTestAndApplyInput))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *freeIpaTestAndApplyInputLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*FreeIpaTestAndApplyInput))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewFreeIpaTestAndApplyInputLifecycleAdapter(name string, clusterScoped bool, client FreeIpaTestAndApplyInputInterface, l FreeIpaTestAndApplyInputLifecycle) FreeIpaTestAndApplyInputHandlerFunc {
	adapter := &freeIpaTestAndApplyInputLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *FreeIpaTestAndApplyInput) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
