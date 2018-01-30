package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GithubConfigLifecycle interface {
	Create(obj *GithubConfig) (*GithubConfig, error)
	Remove(obj *GithubConfig) (*GithubConfig, error)
	Updated(obj *GithubConfig) (*GithubConfig, error)
}

type githubConfigLifecycleAdapter struct {
	lifecycle GithubConfigLifecycle
}

func (w *githubConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GithubConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *githubConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GithubConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *githubConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GithubConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGithubConfigLifecycleAdapter(name string, clusterScoped bool, client GithubConfigInterface, l GithubConfigLifecycle) GithubConfigHandlerFunc {
	adapter := &githubConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *GithubConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
