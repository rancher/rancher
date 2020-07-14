package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NotifierLifecycle interface {
	Create(obj *v3.Notifier) (runtime.Object, error)
	Remove(obj *v3.Notifier) (runtime.Object, error)
	Updated(obj *v3.Notifier) (runtime.Object, error)
}

type notifierLifecycleAdapter struct {
	lifecycle NotifierLifecycle
}

func (w *notifierLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *notifierLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *notifierLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Notifier))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *notifierLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Notifier))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *notifierLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Notifier))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNotifierLifecycleAdapter(name string, clusterScoped bool, client NotifierInterface, l NotifierLifecycle) NotifierHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NotifierGroupVersionResource)
	}
	adapter := &notifierLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Notifier) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
