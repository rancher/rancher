package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type EtcdBackupLifecycle interface {
	Create(obj *EtcdBackup) (runtime.Object, error)
	Remove(obj *EtcdBackup) (runtime.Object, error)
	Updated(obj *EtcdBackup) (runtime.Object, error)
}

type etcdBackupLifecycleAdapter struct {
	lifecycle EtcdBackupLifecycle
}

func (w *etcdBackupLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *etcdBackupLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *etcdBackupLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*EtcdBackup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *etcdBackupLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*EtcdBackup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *etcdBackupLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*EtcdBackup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewEtcdBackupLifecycleAdapter(name string, clusterScoped bool, client EtcdBackupInterface, l EtcdBackupLifecycle) EtcdBackupHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(EtcdBackupGroupVersionResource)
	}
	adapter := &etcdBackupLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *EtcdBackup) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
