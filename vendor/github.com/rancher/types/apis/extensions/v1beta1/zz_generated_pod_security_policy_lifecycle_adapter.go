package v1beta1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityPolicyLifecycle interface {
	Create(obj *v1beta1.PodSecurityPolicy) error
	Remove(obj *v1beta1.PodSecurityPolicy) error
	Updated(obj *v1beta1.PodSecurityPolicy) error
}

type podSecurityPolicyLifecycleAdapter struct {
	lifecycle PodSecurityPolicyLifecycle
}

func (w *podSecurityPolicyLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*v1beta1.PodSecurityPolicy))
}

func (w *podSecurityPolicyLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*v1beta1.PodSecurityPolicy))
}

func (w *podSecurityPolicyLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*v1beta1.PodSecurityPolicy))
}

func NewPodSecurityPolicyLifecycleAdapter(name string, client PodSecurityPolicyInterface, l PodSecurityPolicyLifecycle) PodSecurityPolicyHandlerFunc {
	adapter := &podSecurityPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1beta1.PodSecurityPolicy) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
