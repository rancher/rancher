package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type PipelineSettingLifecycle interface {
	Create(obj *PipelineSetting) (runtime.Object, error)
	Remove(obj *PipelineSetting) (runtime.Object, error)
	Updated(obj *PipelineSetting) (runtime.Object, error)
}

type pipelineSettingLifecycleAdapter struct {
	lifecycle PipelineSettingLifecycle
}

func (w *pipelineSettingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *pipelineSettingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *pipelineSettingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*PipelineSetting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineSettingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*PipelineSetting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineSettingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*PipelineSetting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPipelineSettingLifecycleAdapter(name string, clusterScoped bool, client PipelineSettingInterface, l PipelineSettingLifecycle) PipelineSettingHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PipelineSettingGroupVersionResource)
	}
	adapter := &pipelineSettingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *PipelineSetting) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
