package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type LdapConfigLifecycle interface {
	Create(obj *LdapConfig) (runtime.Object, error)
	Remove(obj *LdapConfig) (runtime.Object, error)
	Updated(obj *LdapConfig) (runtime.Object, error)
}

type ldapConfigLifecycleAdapter struct {
	lifecycle LdapConfigLifecycle
}

func (w *ldapConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *ldapConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(LdapConfigGroupVersionResource)
	}
	adapter := &ldapConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *LdapConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
