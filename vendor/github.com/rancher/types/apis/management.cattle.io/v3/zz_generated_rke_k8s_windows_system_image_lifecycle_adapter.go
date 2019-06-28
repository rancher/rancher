package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type RKEK8sWindowsSystemImageLifecycle interface {
	Create(obj *RKEK8sWindowsSystemImage) (runtime.Object, error)
	Remove(obj *RKEK8sWindowsSystemImage) (runtime.Object, error)
	Updated(obj *RKEK8sWindowsSystemImage) (runtime.Object, error)
}

type rkeK8sWindowsSystemImageLifecycleAdapter struct {
	lifecycle RKEK8sWindowsSystemImageLifecycle
}

func (w *rkeK8sWindowsSystemImageLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *rkeK8sWindowsSystemImageLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *rkeK8sWindowsSystemImageLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*RKEK8sWindowsSystemImage))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sWindowsSystemImageLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*RKEK8sWindowsSystemImage))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sWindowsSystemImageLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*RKEK8sWindowsSystemImage))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRKEK8sWindowsSystemImageLifecycleAdapter(name string, clusterScoped bool, client RKEK8sWindowsSystemImageInterface, l RKEK8sWindowsSystemImageLifecycle) RKEK8sWindowsSystemImageHandlerFunc {
	adapter := &rkeK8sWindowsSystemImageLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *RKEK8sWindowsSystemImage) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
