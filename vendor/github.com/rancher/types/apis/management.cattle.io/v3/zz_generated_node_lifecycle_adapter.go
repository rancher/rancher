package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeLifecycle interface {
	Create(obj *Node) (*Node, error)
	Remove(obj *Node) (*Node, error)
	Updated(obj *Node) (*Node, error)
}

type nodeLifecycleAdapter struct {
	lifecycle NodeLifecycle
}

func (w *nodeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodeLifecycleAdapter(name string, clusterScoped bool, client NodeInterface, l NodeLifecycle) NodeHandlerFunc {
	adapter := &nodeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Node) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
