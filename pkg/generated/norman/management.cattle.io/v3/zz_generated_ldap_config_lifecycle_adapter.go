package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type LdapConfigLifecycle interface {
	Create(obj *v3.LdapConfig) (runtime.Object, error)
	Remove(obj *v3.LdapConfig) (runtime.Object, error)
	Updated(obj *v3.LdapConfig) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*v3.LdapConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *ldapConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.LdapConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *ldapConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.LdapConfig))
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
	return func(key string, obj *v3.LdapConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
