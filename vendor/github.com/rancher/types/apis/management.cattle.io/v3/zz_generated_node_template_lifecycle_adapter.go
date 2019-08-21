package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeTemplateLifecycle interface {
	Create(obj *NodeTemplate) (runtime.Object, error)
	Remove(obj *NodeTemplate) (runtime.Object, error)
	Updated(obj *NodeTemplate) (runtime.Object, error)
}

type nodeTemplateLifecycleAdapter struct {
	lifecycle NodeTemplateLifecycle
}

func (w *nodeTemplateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *nodeTemplateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *nodeTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NodeTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NodeTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NodeTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNodeTemplateLifecycleAdapter(name string, clusterScoped bool, client NodeTemplateInterface, l NodeTemplateLifecycle) NodeTemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NodeTemplateGroupVersionResource)
	}
	adapter := &nodeTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NodeTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
