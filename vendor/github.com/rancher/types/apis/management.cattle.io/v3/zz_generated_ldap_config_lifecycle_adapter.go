package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type LdapConfigLifecycle interface {
	Create(obj *LdapConfig) (*LdapConfig, error)
	Remove(obj *LdapConfig) (*LdapConfig, error)
	Updated(obj *LdapConfig) (*LdapConfig, error)
}

type ldapConfigLifecycleAdapter struct {
	lifecycle LdapConfigLifecycle
}

func (w *ldapConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*LdapConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *ldapConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*LdapConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *ldapConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*LdapConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewLdapConfigLifecycleAdapter(name string, clusterScoped bool, client LdapConfigInterface, l LdapConfigLifecycle) LdapConfigHandlerFunc {
	adapter := &ldapConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *LdapConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
