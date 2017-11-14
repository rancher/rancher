package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type LocalCredentialLifecycle interface {
	Create(obj *LocalCredential) error
	Remove(obj *LocalCredential) error
	Updated(obj *LocalCredential) error
}

type localCredentialLifecycleAdapter struct {
	lifecycle LocalCredentialLifecycle
}

func (w *localCredentialLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*LocalCredential))
}

func (w *localCredentialLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*LocalCredential))
}

func (w *localCredentialLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*LocalCredential))
}

func NewLocalCredentialLifecycleAdapter(name string, client LocalCredentialInterface, l LocalCredentialLifecycle) LocalCredentialHandlerFunc {
	adapter := &localCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *LocalCredential) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
