package v1beta1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyLifecycle interface {
	Create(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)
	Remove(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)
	Updated(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)
}

type podSecurityPolicyLifecycleAdapter struct {
	lifecycle PodSecurityPolicyLifecycle
}

func (w *podSecurityPolicyLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *podSecurityPolicyLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *podSecurityPolicyLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1beta1.PodSecurityPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityPolicyLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1beta1.PodSecurityPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityPolicyLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1beta1.PodSecurityPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPodSecurityPolicyLifecycleAdapter(name string, clusterScoped bool, client PodSecurityPolicyInterface, l PodSecurityPolicyLifecycle) PodSecurityPolicyHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PodSecurityPolicyGroupVersionResource)
	}
	adapter := &podSecurityPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
