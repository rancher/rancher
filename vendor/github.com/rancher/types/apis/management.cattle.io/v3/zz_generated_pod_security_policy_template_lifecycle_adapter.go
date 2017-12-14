package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyTemplateLifecycle interface {
	Create(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Remove(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Updated(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
}

type podSecurityPolicyTemplateLifecycleAdapter struct {
	lifecycle PodSecurityPolicyTemplateLifecycle
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*PodSecurityPolicyTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*PodSecurityPolicyTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*PodSecurityPolicyTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPodSecurityPolicyTemplateLifecycleAdapter(name string, client PodSecurityPolicyTemplateInterface, l PodSecurityPolicyTemplateLifecycle) PodSecurityPolicyTemplateHandlerFunc {
	adapter := &podSecurityPolicyTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *PodSecurityPolicyTemplate) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
