package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type OpenLdapTestAndApplyInputLifecycle interface {
	Create(obj *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error)
	Remove(obj *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error)
	Updated(obj *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error)
}

type openLdapTestAndApplyInputLifecycleAdapter struct {
	lifecycle OpenLdapTestAndApplyInputLifecycle
}

func (w *openLdapTestAndApplyInputLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*OpenLdapTestAndApplyInput))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *openLdapTestAndApplyInputLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*OpenLdapTestAndApplyInput))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *openLdapTestAndApplyInputLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*OpenLdapTestAndApplyInput))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewOpenLdapTestAndApplyInputLifecycleAdapter(name string, clusterScoped bool, client OpenLdapTestAndApplyInputInterface, l OpenLdapTestAndApplyInputLifecycle) OpenLdapTestAndApplyInputHandlerFunc {
	adapter := &openLdapTestAndApplyInputLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *OpenLdapTestAndApplyInput) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
