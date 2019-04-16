package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeLifecycle interface {
	Create(obj *v1.Node) (*v1.Node, error)
	Remove(obj *v1.Node) (*v1.Node, error)
	Updated(obj *v1.Node) (*v1.Node, error)
}

type nodeLifecycleAdapter struct {
	lifecycle NodeLifecycle
}

func (w *nodeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodeLifecycleAdapter(name string, clusterScoped bool, client NodeInterface, l NodeLifecycle) NodeHandlerFunc {
	adapter := &nodeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Node) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
