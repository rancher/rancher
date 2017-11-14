package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodLifecycle interface {
	Create(obj *v1.Pod) error
	Remove(obj *v1.Pod) error
	Updated(obj *v1.Pod) error
}

type podLifecycleAdapter struct {
	lifecycle PodLifecycle
}

func (w *podLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*v1.Pod))
}

func (w *podLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*v1.Pod))
}

func (w *podLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*v1.Pod))
}

func NewPodLifecycleAdapter(name string, client PodInterface, l PodLifecycle) PodHandlerFunc {
	adapter := &podLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.Pod) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
