package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeTemplateLifecycle interface {
	Create(obj *v3.NodeTemplate) (runtime.Object, error)
	Remove(obj *v3.NodeTemplate) (runtime.Object, error)
	Updated(obj *v3.NodeTemplate) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*v3.NodeTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.NodeTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *nodeTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.NodeTemplate))
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
	return func(key string, obj *v3.NodeTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
