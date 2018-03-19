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
	GlobalComposeConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalComposeConfig",
	}
	GlobalComposeConfigResource = metav1.APIResource{
		Name:         "globalcomposeconfigs",
		SingularName: "globalcomposeconfig",
		Namespaced:   false,
		Kind:         GlobalComposeConfigGroupVersionKind.Kind,
	}
)

type GlobalComposeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalComposeConfig
}

type GlobalComposeConfigHandlerFunc func(key string, obj *GlobalComposeConfig) error

type GlobalComposeConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*GlobalComposeConfig, err error)
	Get(namespace, name string) (*GlobalComposeConfig, error)
}

type GlobalComposeConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() GlobalComposeConfigLister
	AddHandler(name string, handler GlobalComposeConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler GlobalComposeConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GlobalComposeConfigInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*GlobalComposeConfig) (*GlobalComposeConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalComposeConfig, error)
	Get(name string, opts metav1.GetOptions) (*GlobalComposeConfig, error)
	Update(*GlobalComposeConfig) (*GlobalComposeConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GlobalComposeConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalComposeConfigController
	AddHandler(name string, sync GlobalComposeConfigHandlerFunc)
	AddLifecycle(name string, lifecycle GlobalComposeConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync GlobalComposeConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle GlobalComposeConfigLifecycle)
}

type globalComposeConfigLister struct {
	controller *globalComposeConfigController
}

func (l *globalComposeConfigLister) List(namespace string, selector labels.Selector) (ret []*GlobalComposeConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GlobalComposeConfig))
	})
	return
}

func (l *globalComposeConfigLister) Get(namespace, name string) (*GlobalComposeConfig, error) {
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
			Group:    GlobalComposeConfigGroupVersionKind.Group,
			Resource: "globalComposeConfig",
		}, name)
	}
	return obj.(*GlobalComposeConfig), nil
}

type globalComposeConfigController struct {
	controller.GenericController
}

func (c *globalComposeConfigController) Lister() GlobalComposeConfigLister {
	return &globalComposeConfigLister{
		controller: c,
	}
}

func (c *globalComposeConfigController) AddHandler(name string, handler GlobalComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*GlobalComposeConfig))
	})
}

func (c *globalComposeConfigController) AddClusterScopedHandler(name, cluster string, handler GlobalComposeConfigHandlerFunc) {
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

		return handler(key, obj.(*GlobalComposeConfig))
	})
}

type globalComposeConfigFactory struct {
}

func (c globalComposeConfigFactory) Object() runtime.Object {
	return &GlobalComposeConfig{}
}

func (c globalComposeConfigFactory) List() runtime.Object {
	return &GlobalComposeConfigList{}
}

func (s *globalComposeConfigClient) Controller() GlobalComposeConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.globalComposeConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GlobalComposeConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &globalComposeConfigController{
		GenericController: genericController,
	}

	s.client.globalComposeConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type globalComposeConfigClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   GlobalComposeConfigController
}

func (s *globalComposeConfigClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *globalComposeConfigClient) Create(o *GlobalComposeConfig) (*GlobalComposeConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GlobalComposeConfig), err
}

func (s *globalComposeConfigClient) Get(name string, opts metav1.GetOptions) (*GlobalComposeConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GlobalComposeConfig), err
}

func (s *globalComposeConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalComposeConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*GlobalComposeConfig), err
}

func (s *globalComposeConfigClient) Update(o *GlobalComposeConfig) (*GlobalComposeConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GlobalComposeConfig), err
}

func (s *globalComposeConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalComposeConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalComposeConfigClient) List(opts metav1.ListOptions) (*GlobalComposeConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GlobalComposeConfigList), err
}

func (s *globalComposeConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalComposeConfigClient) Patch(o *GlobalComposeConfig, data []byte, subresources ...string) (*GlobalComposeConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*GlobalComposeConfig), err
}

func (s *globalComposeConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalComposeConfigClient) AddHandler(name string, sync GlobalComposeConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *globalComposeConfigClient) AddLifecycle(name string, lifecycle GlobalComposeConfigLifecycle) {
	sync := NewGlobalComposeConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *globalComposeConfigClient) AddClusterScopedHandler(name, clusterName string, sync GlobalComposeConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *globalComposeConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle GlobalComposeConfigLifecycle) {
	sync := NewGlobalComposeConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
