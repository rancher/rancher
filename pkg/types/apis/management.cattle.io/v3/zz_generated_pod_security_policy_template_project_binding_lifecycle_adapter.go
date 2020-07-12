package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyTemplateProjectBindingLifecycle interface {
	Create(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error)
	Remove(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error)
	Updated(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error)
}

type podSecurityPolicyTemplateProjectBindingLifecycleAdapter struct {
	lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle
}

func (w *podSecurityPolicyTemplateProjectBindingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *podSecurityPolicyTemplateProjectBindingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(PodSecurityPolicyTemplateProjectBindingGroupVersionResource)
	}
	adapter := &podSecurityPolicyTemplateProjectBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
