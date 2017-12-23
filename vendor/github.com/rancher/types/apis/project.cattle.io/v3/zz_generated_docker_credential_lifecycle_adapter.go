package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type DockerCredentialLifecycle interface {
	Create(obj *DockerCredential) (*DockerCredential, error)
	Remove(obj *DockerCredential) (*DockerCredential, error)
	Updated(obj *DockerCredential) (*DockerCredential, error)
}

type dockerCredentialLifecycleAdapter struct {
	lifecycle DockerCredentialLifecycle
}

func (w *dockerCredentialLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*DockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *dockerCredentialLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*DockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *dockerCredentialLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*DockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewDockerCredentialLifecycleAdapter(name string, client DockerCredentialInterface, l DockerCredentialLifecycle) DockerCredentialHandlerFunc {
	adapter := &dockerCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *DockerCredential) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
