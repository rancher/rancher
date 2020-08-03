package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeLifecycle interface {
	Create(obj *v3.Node) (runtime.Object, error)
	Remove(obj *v3.Node) (runtime.Object, error)
	Updated(obj *v3.Node) (runtime.Object, error)
}

type nodeLifecycleAdapter struct {
	lifecycle NodeLifecycle
}

func (w *nodeLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *nodeLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *nodeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Node))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodeLifecycleAdapter(name string, clusterScoped bool, client NodeInterface, l NodeLifecycle) NodeHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NodeGroupVersionResource)
	}
	adapter := &nodeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Node) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
