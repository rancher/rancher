package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeDriverLifecycle interface {
	Create(obj *NodeDriver) (runtime.Object, error)
	Remove(obj *NodeDriver) (runtime.Object, error)
	Updated(obj *NodeDriver) (runtime.Object, error)
}

type nodeDriverLifecycleAdapter struct {
	lifecycle NodeDriverLifecycle
}

func (w *nodeDriverLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *nodeDriverLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(NodeDriverGroupVersionResource)
	}
	adapter := &nodeDriverLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NodeDriver) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
