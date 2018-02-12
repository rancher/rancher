package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NotifierLifecycle interface {
	Create(obj *Notifier) (*Notifier, error)
	Remove(obj *Notifier) (*Notifier, error)
	Updated(obj *Notifier) (*Notifier, error)
}

type notifierLifecycleAdapter struct {
	lifecycle NotifierLifecycle
}

func (w *notifierLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Notifier))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *notifierLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Notifier))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *notifierLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Notifier))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNotifierLifecycleAdapter(name string, clusterScoped bool, client NotifierInterface, l NotifierLifecycle) NotifierHandlerFunc {
	adapter := &notifierLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Notifier) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
