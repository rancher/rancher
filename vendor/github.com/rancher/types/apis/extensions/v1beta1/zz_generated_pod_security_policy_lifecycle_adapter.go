package v1beta1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyLifecycle interface {
	Create(obj *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Remove(obj *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Updated(obj *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
}

type podSecurityPolicyLifecycleAdapter struct {
	lifecycle PodSecurityPolicyLifecycle
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
	adapter := &podSecurityPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta1.PodSecurityPolicy) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
