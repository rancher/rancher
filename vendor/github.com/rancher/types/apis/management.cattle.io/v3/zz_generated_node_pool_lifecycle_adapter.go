package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodePoolLifecycle interface {
	Create(obj *NodePool) (*NodePool, error)
	Remove(obj *NodePool) (*NodePool, error)
	Updated(obj *NodePool) (*NodePool, error)
}

type nodePoolLifecycleAdapter struct {
	lifecycle NodePoolLifecycle
}

func (w *nodePoolLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NodePool))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodePoolLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NodePool))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodePoolLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NodePool))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodePoolLifecycleAdapter(name string, clusterScoped bool, client NodePoolInterface, l NodePoolLifecycle) NodePoolHandlerFunc {
	adapter := &nodePoolLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NodePool) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
