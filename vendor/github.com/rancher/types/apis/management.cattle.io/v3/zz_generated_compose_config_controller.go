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
	ComposeConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ComposeConfig",
	}
	ComposeConfigResource = metav1.APIResource{
		Name:         "composeconfigs",
		SingularName: "composeconfig",
		Namespaced:   false,
		Kind:         ComposeConfigGroupVersionKind.Kind,
	}
)

type ComposeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComposeConfig
}

type ComposeConfigHandlerFunc func(key string, obj *ComposeConfig) error

type ComposeConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*ComposeConfig, err error)
	Get(namespace, name string) (*ComposeConfig, error)
}

type ComposeConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ComposeConfigLister
	AddHandler(name string, handler ComposeConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ComposeConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ComposeConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ComposeConfig) (*ComposeConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ComposeConfig, error)
	Get(name string, opts metav1.GetOptions) (*ComposeConfig, error)
	Update(*ComposeConfig) (*ComposeConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ComposeConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ComposeConfigController
	AddHandler(name string, sync ComposeConfigHandlerFunc)
	AddLifecycle(name string, lifecycle ComposeConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ComposeConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ComposeConfigLifecycle)
}

type composeConfigLister struct {
	controller *composeConfigController
}

func (l *composeConfigLister) List(namespace string, selector labels.Selector) (ret []*ComposeConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ComposeConfig))
	})
	return
}

func (l *composeConfigLister) Get(namespace, name string) (*ComposeConfig, error) {
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
			Group:    ComposeConfigGroupVersionKind.Group,
			Resource: "composeConfig",
		}, key)
	}
	return obj.(*ComposeConfig), nil
}

type composeConfigController struct {
	controller.GenericController
}

func (c *composeConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *composeConfigController) Lister() ComposeConfigLister {
	return &composeConfigLister{
		controller: c,
	}
}

func (c *composeConfigController) AddHandler(name string, handler ComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ComposeConfig))
	})
}

func (c *composeConfigController) AddClusterScopedHandler(name, cluster string, handler ComposeConfigHandlerFunc) {
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

		return handler(key, obj.(*ComposeConfig))
	})
}

type composeConfigFactory struct {
}

func (c composeConfigFactory) Object() runtime.Object {
	return &ComposeConfig{}
}

func (c composeConfigFactory) List() runtime.Object {
	return &ComposeConfigList{}
}

func (s *composeConfigClient) Controller() ComposeConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.composeConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ComposeConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &composeConfigController{
		GenericController: genericController,
	}

	s.client.composeConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type composeConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ComposeConfigController
}

func (s *composeConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *composeConfigClient) Create(o *ComposeConfig) (*ComposeConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ComposeConfig), err
}

func (s *composeConfigClient) Get(name string, opts metav1.GetOptions) (*ComposeConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ComposeConfig), err
}

func (s *composeConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ComposeConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ComposeConfig), err
}

func (s *composeConfigClient) Update(o *ComposeConfig) (*ComposeConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ComposeConfig), err
}

func (s *composeConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *composeConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *composeConfigClient) List(opts metav1.ListOptions) (*ComposeConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ComposeConfigList), err
}

func (s *composeConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *composeConfigClient) Patch(o *ComposeConfig, data []byte, subresources ...string) (*ComposeConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ComposeConfig), err
}

func (s *composeConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *composeConfigClient) AddHandler(name string, sync ComposeConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *composeConfigClient) AddLifecycle(name string, lifecycle ComposeConfigLifecycle) {
	sync := NewComposeConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *composeConfigClient) AddClusterScopedHandler(name, clusterName string, sync ComposeConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *composeConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ComposeConfigLifecycle) {
	sync := NewComposeConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
