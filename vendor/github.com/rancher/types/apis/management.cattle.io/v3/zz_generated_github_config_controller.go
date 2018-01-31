package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	GithubConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GithubConfig",
	}
	GithubConfigResource = metav1.APIResource{
		Name:         "githubconfigs",
		SingularName: "githubconfig",
		Namespaced:   false,
		Kind:         GithubConfigGroupVersionKind.Kind,
	}
)

type GithubConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GithubConfig
}

type GithubConfigHandlerFunc func(key string, obj *GithubConfig) error

type GithubConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*GithubConfig, err error)
	Get(namespace, name string) (*GithubConfig, error)
}

type GithubConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() GithubConfigLister
	AddHandler(name string, handler GithubConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler GithubConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GithubConfigInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*GithubConfig) (*GithubConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GithubConfig, error)
	Get(name string, opts metav1.GetOptions) (*GithubConfig, error)
	Update(*GithubConfig) (*GithubConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GithubConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GithubConfigController
	AddHandler(name string, sync GithubConfigHandlerFunc)
	AddLifecycle(name string, lifecycle GithubConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync GithubConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle GithubConfigLifecycle)
}

type githubConfigLister struct {
	controller *githubConfigController
}

func (l *githubConfigLister) List(namespace string, selector labels.Selector) (ret []*GithubConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GithubConfig))
	})
	return
}

func (l *githubConfigLister) Get(namespace, name string) (*GithubConfig, error) {
	var key string
	if namespace != "" {
		key = namespace + "/" + name
	} else {
		key = name
	}
	obj, exists, err := l.controller.Informer().GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    GithubConfigGroupVersionKind.Group,
			Resource: "githubConfig",
		}, name)
	}
	return obj.(*GithubConfig), nil
}

type githubConfigController struct {
	controller.GenericController
}

func (c *githubConfigController) Lister() GithubConfigLister {
	return &githubConfigLister{
		controller: c,
	}
}

func (c *githubConfigController) AddHandler(name string, handler GithubConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*GithubConfig))
	})
}

func (c *githubConfigController) AddClusterScopedHandler(name, cluster string, handler GithubConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*GithubConfig))
	})
}

type githubConfigFactory struct {
}

func (c githubConfigFactory) Object() runtime.Object {
	return &GithubConfig{}
}

func (c githubConfigFactory) List() runtime.Object {
	return &GithubConfigList{}
}

func (s *githubConfigClient) Controller() GithubConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.githubConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GithubConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &githubConfigController{
		GenericController: genericController,
	}

	s.client.githubConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type githubConfigClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   GithubConfigController
}

func (s *githubConfigClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *githubConfigClient) Create(o *GithubConfig) (*GithubConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GithubConfig), err
}

func (s *githubConfigClient) Get(name string, opts metav1.GetOptions) (*GithubConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GithubConfig), err
}

func (s *githubConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GithubConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*GithubConfig), err
}

func (s *githubConfigClient) Update(o *GithubConfig) (*GithubConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GithubConfig), err
}

func (s *githubConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *githubConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *githubConfigClient) List(opts metav1.ListOptions) (*GithubConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GithubConfigList), err
}

func (s *githubConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *githubConfigClient) Patch(o *GithubConfig, data []byte, subresources ...string) (*GithubConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*GithubConfig), err
}

func (s *githubConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *githubConfigClient) AddHandler(name string, sync GithubConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *githubConfigClient) AddLifecycle(name string, lifecycle GithubConfigLifecycle) {
	sync := NewGithubConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *githubConfigClient) AddClusterScopedHandler(name, clusterName string, sync GithubConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *githubConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle GithubConfigLifecycle) {
	sync := NewGithubConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
