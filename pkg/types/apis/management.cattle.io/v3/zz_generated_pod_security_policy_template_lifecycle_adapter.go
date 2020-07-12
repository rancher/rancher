package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyTemplateLifecycle interface {
	Create(obj *PodSecurityPolicyTemplate) (runtime.Object, error)
	Remove(obj *PodSecurityPolicyTemplate) (runtime.Object, error)
	Updated(obj *PodSecurityPolicyTemplate) (runtime.Object, error)
}

type podSecurityPolicyTemplateLifecycleAdapter struct {
	lifecycle PodSecurityPolicyTemplateLifecycle
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *podSecurityPolicyTemplateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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

func NewPodSecurityPolicyTemplateLifecycleAdapter(name string, clusterScoped bool, client PodSecurityPolicyTemplateInterface, l PodSecurityPolicyTemplateLifecycle) PodSecurityPolicyTemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PodSecurityPolicyTemplateGroupVersionResource)
	}
	adapter := &podSecurityPolicyTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *PodSecurityPolicyTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
