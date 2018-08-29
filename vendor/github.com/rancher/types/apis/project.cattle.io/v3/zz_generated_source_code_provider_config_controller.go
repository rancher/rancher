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
	SourceCodeProviderConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeProviderConfig",
	}
	SourceCodeProviderConfigResource = metav1.APIResource{
		Name:         "sourcecodeproviderconfigs",
		SingularName: "sourcecodeproviderconfig",
		Namespaced:   true,

		Kind: SourceCodeProviderConfigGroupVersionKind.Kind,
	}
)

type SourceCodeProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeProviderConfig
}

type SourceCodeProviderConfigHandlerFunc func(key string, obj *SourceCodeProviderConfig) error

type SourceCodeProviderConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeProviderConfig, err error)
	Get(namespace, name string) (*SourceCodeProviderConfig, error)
}

type SourceCodeProviderConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeProviderConfigLister
	AddHandler(name string, handler SourceCodeProviderConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler SourceCodeProviderConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeProviderConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error)
	Update(*SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeProviderConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeProviderConfigController
	AddHandler(name string, sync SourceCodeProviderConfigHandlerFunc)
	AddLifecycle(name string, lifecycle SourceCodeProviderConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync SourceCodeProviderConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeProviderConfigLifecycle)
}

type sourceCodeProviderConfigLister struct {
	controller *sourceCodeProviderConfigController
}

func (l *sourceCodeProviderConfigLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeProviderConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeProviderConfig))
	})
	return
}

func (l *sourceCodeProviderConfigLister) Get(namespace, name string) (*SourceCodeProviderConfig, error) {
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
			Group:    SourceCodeProviderConfigGroupVersionKind.Group,
			Resource: "sourceCodeProviderConfig",
		}, key)
	}
	return obj.(*SourceCodeProviderConfig), nil
}

type sourceCodeProviderConfigController struct {
	controller.GenericController
}

func (c *sourceCodeProviderConfigController) Lister() SourceCodeProviderConfigLister {
	return &sourceCodeProviderConfigLister{
		controller: c,
	}
}

func (c *sourceCodeProviderConfigController) AddHandler(name string, handler SourceCodeProviderConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*SourceCodeProviderConfig))
	})
}

func (c *sourceCodeProviderConfigController) AddClusterScopedHandler(name, cluster string, handler SourceCodeProviderConfigHandlerFunc) {
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

		return handler(key, obj.(*SourceCodeProviderConfig))
	})
}

type sourceCodeProviderConfigFactory struct {
}

func (c sourceCodeProviderConfigFactory) Object() runtime.Object {
	return &SourceCodeProviderConfig{}
}

func (c sourceCodeProviderConfigFactory) List() runtime.Object {
	return &SourceCodeProviderConfigList{}
}

func (s *sourceCodeProviderConfigClient) Controller() SourceCodeProviderConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeProviderConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeProviderConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeProviderConfigController{
		GenericController: genericController,
	}

	s.client.sourceCodeProviderConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeProviderConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeProviderConfigController
}

func (s *sourceCodeProviderConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeProviderConfigClient) Create(o *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Get(name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Update(o *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeProviderConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeProviderConfigClient) List(opts metav1.ListOptions) (*SourceCodeProviderConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeProviderConfigList), err
}

func (s *sourceCodeProviderConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeProviderConfigClient) Patch(o *SourceCodeProviderConfig, data []byte, subresources ...string) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeProviderConfigClient) AddHandler(name string, sync SourceCodeProviderConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *sourceCodeProviderConfigClient) AddLifecycle(name string, lifecycle SourceCodeProviderConfigLifecycle) {
	sync := NewSourceCodeProviderConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *sourceCodeProviderConfigClient) AddClusterScopedHandler(name, clusterName string, sync SourceCodeProviderConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *sourceCodeProviderConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeProviderConfigLifecycle) {
	sync := NewSourceCodeProviderConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
