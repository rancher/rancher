package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeTemplateLifecycle interface {
	Create(obj *NodeTemplate) (*NodeTemplate, error)
	Remove(obj *NodeTemplate) (*NodeTemplate, error)
	Updated(obj *NodeTemplate) (*NodeTemplate, error)
}

type nodeTemplateLifecycleAdapter struct {
	lifecycle NodeTemplateLifecycle
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
	adapter := &nodeTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NodeTemplate) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
