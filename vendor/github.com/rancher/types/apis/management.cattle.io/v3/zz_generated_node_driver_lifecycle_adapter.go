package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeDriverLifecycle interface {
	Create(obj *NodeDriver) (*NodeDriver, error)
	Remove(obj *NodeDriver) (*NodeDriver, error)
	Updated(obj *NodeDriver) (*NodeDriver, error)
}

type nodeDriverLifecycleAdapter struct {
	lifecycle NodeDriverLifecycle
}

func (w *nodeDriverLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NodeDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeDriverLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NodeDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeDriverLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NodeDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodeDriverLifecycleAdapter(name string, clusterScoped bool, client NodeDriverInterface, l NodeDriverLifecycle) NodeDriverHandlerFunc {
	adapter := &nodeDriverLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NodeDriver) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
