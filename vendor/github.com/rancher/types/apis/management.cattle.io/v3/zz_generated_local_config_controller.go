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
	LocalConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "LocalConfig",
	}
	LocalConfigResource = metav1.APIResource{
		Name:         "localconfigs",
		SingularName: "localconfig",
		Namespaced:   false,
		Kind:         LocalConfigGroupVersionKind.Kind,
	}
)

type LocalConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LocalConfig
}

type LocalConfigHandlerFunc func(key string, obj *LocalConfig) error

type LocalConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*LocalConfig, err error)
	Get(namespace, name string) (*LocalConfig, error)
}

type LocalConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() LocalConfigLister
	AddHandler(name string, handler LocalConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler LocalConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type LocalConfigInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*LocalConfig) (*LocalConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*LocalConfig, error)
	Get(name string, opts metav1.GetOptions) (*LocalConfig, error)
	Update(*LocalConfig) (*LocalConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*LocalConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() LocalConfigController
	AddHandler(name string, sync LocalConfigHandlerFunc)
	AddLifecycle(name string, lifecycle LocalConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync LocalConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle LocalConfigLifecycle)
}

type localConfigLister struct {
	controller *localConfigController
}

func (l *localConfigLister) List(namespace string, selector labels.Selector) (ret []*LocalConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*LocalConfig))
	})
	return
}

func (l *localConfigLister) Get(namespace, name string) (*LocalConfig, error) {
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
			Group:    LocalConfigGroupVersionKind.Group,
			Resource: "localConfig",
		}, name)
	}
	return obj.(*LocalConfig), nil
}

type localConfigController struct {
	controller.GenericController
}

func (c *localConfigController) Lister() LocalConfigLister {
	return &localConfigLister{
		controller: c,
	}
}

func (c *localConfigController) AddHandler(name string, handler LocalConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*LocalConfig))
	})
}

func (c *localConfigController) AddClusterScopedHandler(name, cluster string, handler LocalConfigHandlerFunc) {
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

		return handler(key, obj.(*LocalConfig))
	})
}

type localConfigFactory struct {
}

func (c localConfigFactory) Object() runtime.Object {
	return &LocalConfig{}
}

func (c localConfigFactory) List() runtime.Object {
	return &LocalConfigList{}
}

func (s *localConfigClient) Controller() LocalConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.localConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(LocalConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &localConfigController{
		GenericController: genericController,
	}

	s.client.localConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type localConfigClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   LocalConfigController
}

func (s *localConfigClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *localConfigClient) Create(o *LocalConfig) (*LocalConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*LocalConfig), err
}

func (s *localConfigClient) Get(name string, opts metav1.GetOptions) (*LocalConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*LocalConfig), err
}

func (s *localConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*LocalConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*LocalConfig), err
}

func (s *localConfigClient) Update(o *LocalConfig) (*LocalConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*LocalConfig), err
}

func (s *localConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *localConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *localConfigClient) List(opts metav1.ListOptions) (*LocalConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*LocalConfigList), err
}

func (s *localConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *localConfigClient) Patch(o *LocalConfig, data []byte, subresources ...string) (*LocalConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*LocalConfig), err
}

func (s *localConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *localConfigClient) AddHandler(name string, sync LocalConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *localConfigClient) AddLifecycle(name string, lifecycle LocalConfigLifecycle) {
	sync := NewLocalConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *localConfigClient) AddClusterScopedHandler(name, clusterName string, sync LocalConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *localConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle LocalConfigLifecycle) {
	sync := NewLocalConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
