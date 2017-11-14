package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GroupMemberLifecycle interface {
	Create(obj *GroupMember) error
	Remove(obj *GroupMember) error
	Updated(obj *GroupMember) error
}

type groupMemberLifecycleAdapter struct {
	lifecycle GroupMemberLifecycle
}

func (w *groupMemberLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*GroupMember))
}

func (w *groupMemberLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*GroupMember))
}

func (w *groupMemberLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*GroupMember))
}

func NewGroupMemberLifecycleAdapter(name string, client GroupMemberInterface, l GroupMemberLifecycle) GroupMemberHandlerFunc {
	adapter := &groupMemberLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *GroupMember) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
