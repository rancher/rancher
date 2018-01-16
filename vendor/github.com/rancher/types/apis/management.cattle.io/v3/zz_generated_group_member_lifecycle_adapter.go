package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GroupMemberLifecycle interface {
	Create(obj *GroupMember) (*GroupMember, error)
	Remove(obj *GroupMember) (*GroupMember, error)
	Updated(obj *GroupMember) (*GroupMember, error)
}

type groupMemberLifecycleAdapter struct {
	lifecycle GroupMemberLifecycle
}

func (w *groupMemberLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GroupMember))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *groupMemberLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GroupMember))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *groupMemberLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GroupMember))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGroupMemberLifecycleAdapter(name string, clusterScoped bool, client GroupMemberInterface, l GroupMemberLifecycle) GroupMemberHandlerFunc {
	adapter := &groupMemberLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *GroupMember) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
