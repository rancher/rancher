package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GithubCredentialLifecycle interface {
	Create(obj *GithubCredential) error
	Remove(obj *GithubCredential) error
	Updated(obj *GithubCredential) error
}

type githubCredentialLifecycleAdapter struct {
	lifecycle GithubCredentialLifecycle
}

func (w *githubCredentialLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*GithubCredential))
}

func (w *githubCredentialLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*GithubCredential))
}

func (w *githubCredentialLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*GithubCredential))
}

func NewGithubCredentialLifecycleAdapter(name string, client GithubCredentialInterface, l GithubCredentialLifecycle) GithubCredentialHandlerFunc {
	adapter := &githubCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *GithubCredential) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
