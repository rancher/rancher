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
	NamespaceComposeConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespaceComposeConfig",
	}
	NamespaceComposeConfigResource = metav1.APIResource{
		Name:         "namespacecomposeconfigs",
		SingularName: "namespacecomposeconfig",
		Namespaced:   true,

		Kind: NamespaceComposeConfigGroupVersionKind.Kind,
	}
)

type NamespaceComposeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceComposeConfig
}

type NamespaceComposeConfigHandlerFunc func(key string, obj *NamespaceComposeConfig) error

type NamespaceComposeConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespaceComposeConfig, err error)
	Get(namespace, name string) (*NamespaceComposeConfig, error)
}

type NamespaceComposeConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() NamespaceComposeConfigLister
	AddHandler(name string, handler NamespaceComposeConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NamespaceComposeConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespaceComposeConfigInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*NamespaceComposeConfig) (*NamespaceComposeConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespaceComposeConfig, error)
	Get(name string, opts metav1.GetOptions) (*NamespaceComposeConfig, error)
	Update(*NamespaceComposeConfig) (*NamespaceComposeConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespaceComposeConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespaceComposeConfigController
	AddHandler(name string, sync NamespaceComposeConfigHandlerFunc)
	AddLifecycle(name string, lifecycle NamespaceComposeConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NamespaceComposeConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespaceComposeConfigLifecycle)
}

type namespaceComposeConfigLister struct {
	controller *namespaceComposeConfigController
}

func (l *namespaceComposeConfigLister) List(namespace string, selector labels.Selector) (ret []*NamespaceComposeConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespaceComposeConfig))
	})
	return
}

func (l *namespaceComposeConfigLister) Get(namespace, name string) (*NamespaceComposeConfig, error) {
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
			Group:    NamespaceComposeConfigGroupVersionKind.Group,
			Resource: "namespaceComposeConfig",
		}, name)
	}
	return obj.(*NamespaceComposeConfig), nil
}

type namespaceComposeConfigController struct {
	controller.GenericController
}

func (c *namespaceComposeConfigController) Lister() NamespaceComposeConfigLister {
	return &namespaceComposeConfigLister{
		controller: c,
	}
}

func (c *namespaceComposeConfigController) AddHandler(name string, handler NamespaceComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NamespaceComposeConfig))
	})
}

func (c *namespaceComposeConfigController) AddClusterScopedHandler(name, cluster string, handler NamespaceComposeConfigHandlerFunc) {
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

		return handler(key, obj.(*NamespaceComposeConfig))
	})
}

type namespaceComposeConfigFactory struct {
}

func (c namespaceComposeConfigFactory) Object() runtime.Object {
	return &NamespaceComposeConfig{}
}

func (c namespaceComposeConfigFactory) List() runtime.Object {
	return &NamespaceComposeConfigList{}
}

func (s *namespaceComposeConfigClient) Controller() NamespaceComposeConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespaceComposeConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespaceComposeConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespaceComposeConfigController{
		GenericController: genericController,
	}

	s.client.namespaceComposeConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespaceComposeConfigClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   NamespaceComposeConfigController
}

func (s *namespaceComposeConfigClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *namespaceComposeConfigClient) Create(o *NamespaceComposeConfig) (*NamespaceComposeConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespaceComposeConfig), err
}

func (s *namespaceComposeConfigClient) Get(name string, opts metav1.GetOptions) (*NamespaceComposeConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespaceComposeConfig), err
}

func (s *namespaceComposeConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespaceComposeConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespaceComposeConfig), err
}

func (s *namespaceComposeConfigClient) Update(o *NamespaceComposeConfig) (*NamespaceComposeConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespaceComposeConfig), err
}

func (s *namespaceComposeConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespaceComposeConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespaceComposeConfigClient) List(opts metav1.ListOptions) (*NamespaceComposeConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespaceComposeConfigList), err
}

func (s *namespaceComposeConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespaceComposeConfigClient) Patch(o *NamespaceComposeConfig, data []byte, subresources ...string) (*NamespaceComposeConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NamespaceComposeConfig), err
}

func (s *namespaceComposeConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespaceComposeConfigClient) AddHandler(name string, sync NamespaceComposeConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *namespaceComposeConfigClient) AddLifecycle(name string, lifecycle NamespaceComposeConfigLifecycle) {
	sync := NewNamespaceComposeConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *namespaceComposeConfigClient) AddClusterScopedHandler(name, clusterName string, sync NamespaceComposeConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *namespaceComposeConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespaceComposeConfigLifecycle) {
	sync := NewNamespaceComposeConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
