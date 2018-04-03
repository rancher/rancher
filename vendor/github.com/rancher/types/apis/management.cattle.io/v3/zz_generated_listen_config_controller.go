package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ListenConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ListenConfig",
	}
	ListenConfigResource = metav1.APIResource{
		Name:         "listenconfigs",
		SingularName: "listenconfig",
		Namespaced:   false,
		Kind:         ListenConfigGroupVersionKind.Kind,
	}
)

type ListenConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ListenConfig
}

type ListenConfigHandlerFunc func(key string, obj *ListenConfig) error

type ListenConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*ListenConfig, err error)
	Get(namespace, name string) (*ListenConfig, error)
}

type ListenConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() ListenConfigLister
	AddHandler(name string, handler ListenConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ListenConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ListenConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ListenConfig) (*ListenConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ListenConfig, error)
	Get(name string, opts metav1.GetOptions) (*ListenConfig, error)
	Update(*ListenConfig) (*ListenConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ListenConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ListenConfigController
	AddHandler(name string, sync ListenConfigHandlerFunc)
	AddLifecycle(name string, lifecycle ListenConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ListenConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ListenConfigLifecycle)
}

type listenConfigLister struct {
	controller *listenConfigController
}

func (l *listenConfigLister) List(namespace string, selector labels.Selector) (ret []*ListenConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ListenConfig))
	})
	return
}

func (l *listenConfigLister) Get(namespace, name string) (*ListenConfig, error) {
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
			Group:    ListenConfigGroupVersionKind.Group,
			Resource: "listenConfig",
		}, name)
	}
	return obj.(*ListenConfig), nil
}

type listenConfigController struct {
	controller.GenericController
}

func (c *listenConfigController) Lister() ListenConfigLister {
	return &listenConfigLister{
		controller: c,
	}
}

func (c *listenConfigController) AddHandler(name string, handler ListenConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ListenConfig))
	})
}

func (c *listenConfigController) AddClusterScopedHandler(name, cluster string, handler ListenConfigHandlerFunc) {
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

		return handler(key, obj.(*ListenConfig))
	})
}

type listenConfigFactory struct {
}

func (c listenConfigFactory) Object() runtime.Object {
	return &ListenConfig{}
}

func (c listenConfigFactory) List() runtime.Object {
	return &ListenConfigList{}
}

func (s *listenConfigClient) Controller() ListenConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.listenConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ListenConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &listenConfigController{
		GenericController: genericController,
	}

	s.client.listenConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type listenConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ListenConfigController
}

func (s *listenConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *listenConfigClient) Create(o *ListenConfig) (*ListenConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) Get(name string, opts metav1.GetOptions) (*ListenConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ListenConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) Update(o *ListenConfig) (*ListenConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *listenConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *listenConfigClient) List(opts metav1.ListOptions) (*ListenConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ListenConfigList), err
}

func (s *listenConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *listenConfigClient) Patch(o *ListenConfig, data []byte, subresources ...string) (*ListenConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *listenConfigClient) AddHandler(name string, sync ListenConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *listenConfigClient) AddLifecycle(name string, lifecycle ListenConfigLifecycle) {
	sync := NewListenConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *listenConfigClient) AddClusterScopedHandler(name, clusterName string, sync ListenConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *listenConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ListenConfigLifecycle) {
	sync := NewListenConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
