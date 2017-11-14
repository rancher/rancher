package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyTemplateLifecycle interface {
	Create(obj *PodSecurityPolicyTemplate) error
	Remove(obj *PodSecurityPolicyTemplate) error
	Updated(obj *PodSecurityPolicyTemplate) error
}

type podSecurityPolicyTemplateLifecycleAdapter struct {
	lifecycle PodSecurityPolicyTemplateLifecycle
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*PodSecurityPolicyTemplate))
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*PodSecurityPolicyTemplate))
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*PodSecurityPolicyTemplate))
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
