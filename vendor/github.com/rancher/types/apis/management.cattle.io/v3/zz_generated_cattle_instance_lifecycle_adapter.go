package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type CattleInstanceLifecycle interface {
	Create(obj *CattleInstance) (*CattleInstance, error)
	Remove(obj *CattleInstance) (*CattleInstance, error)
	Updated(obj *CattleInstance) (*CattleInstance, error)
}

type cattleInstanceLifecycleAdapter struct {
	lifecycle CattleInstanceLifecycle
}

func (w *cattleInstanceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*CattleInstance))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cattleInstanceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*CattleInstance))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cattleInstanceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*CattleInstance))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCattleInstanceLifecycleAdapter(name string, clusterScoped bool, client CattleInstanceInterface, l CattleInstanceLifecycle) CattleInstanceHandlerFunc {
	adapter := &cattleInstanceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *CattleInstance) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
