package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyTemplateProjectBindingLifecycle interface {
	Create(obj *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Remove(obj *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Updated(obj *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
}

type podSecurityPolicyTemplateProjectBindingLifecycleAdapter struct {
	lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle
}

func (w *podSecurityPolicyTemplateProjectBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*PodSecurityPolicyTemplateProjectBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityPolicyTemplateProjectBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*PodSecurityPolicyTemplateProjectBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityPolicyTemplateProjectBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*PodSecurityPolicyTemplateProjectBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name string, clusterScoped bool, client PodSecurityPolicyTemplateProjectBindingInterface, l PodSecurityPolicyTemplateProjectBindingLifecycle) PodSecurityPolicyTemplateProjectBindingHandlerFunc {
	adapter := &podSecurityPolicyTemplateProjectBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *PodSecurityPolicyTemplateProjectBinding) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
