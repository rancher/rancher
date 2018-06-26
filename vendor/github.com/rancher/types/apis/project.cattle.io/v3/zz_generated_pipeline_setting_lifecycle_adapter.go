package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PipelineSettingLifecycle interface {
	Create(obj *PipelineSetting) (*PipelineSetting, error)
	Remove(obj *PipelineSetting) (*PipelineSetting, error)
	Updated(obj *PipelineSetting) (*PipelineSetting, error)
}

type pipelineSettingLifecycleAdapter struct {
	lifecycle PipelineSettingLifecycle
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
	adapter := &pipelineSettingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *PipelineSetting) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
