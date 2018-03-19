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
	ClusterComposeConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterComposeConfig",
	}
	ClusterComposeConfigResource = metav1.APIResource{
		Name:         "clustercomposeconfigs",
		SingularName: "clustercomposeconfig",
		Namespaced:   true,

		Kind: ClusterComposeConfigGroupVersionKind.Kind,
	}
)

type ClusterComposeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterComposeConfig
}

type ClusterComposeConfigHandlerFunc func(key string, obj *ClusterComposeConfig) error

type ClusterComposeConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterComposeConfig, err error)
	Get(namespace, name string) (*ClusterComposeConfig, error)
}

type ClusterComposeConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterComposeConfigLister
	AddHandler(name string, handler ClusterComposeConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterComposeConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterComposeConfigInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*ClusterComposeConfig) (*ClusterComposeConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterComposeConfig, error)
	Get(name string, opts metav1.GetOptions) (*ClusterComposeConfig, error)
	Update(*ClusterComposeConfig) (*ClusterComposeConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterComposeConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterComposeConfigController
	AddHandler(name string, sync ClusterComposeConfigHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterComposeConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterComposeConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterComposeConfigLifecycle)
}

type clusterComposeConfigLister struct {
	controller *clusterComposeConfigController
}

func (l *clusterComposeConfigLister) List(namespace string, selector labels.Selector) (ret []*ClusterComposeConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterComposeConfig))
	})
	return
}

func (l *clusterComposeConfigLister) Get(namespace, name string) (*ClusterComposeConfig, error) {
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
			Group:    ClusterComposeConfigGroupVersionKind.Group,
			Resource: "clusterComposeConfig",
		}, name)
	}
	return obj.(*ClusterComposeConfig), nil
}

type clusterComposeConfigController struct {
	controller.GenericController
}

func (c *clusterComposeConfigController) Lister() ClusterComposeConfigLister {
	return &clusterComposeConfigLister{
		controller: c,
	}
}

func (c *clusterComposeConfigController) AddHandler(name string, handler ClusterComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ClusterComposeConfig))
	})
}

func (c *clusterComposeConfigController) AddClusterScopedHandler(name, cluster string, handler ClusterComposeConfigHandlerFunc) {
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

		return handler(key, obj.(*ClusterComposeConfig))
	})
}

type clusterComposeConfigFactory struct {
}

func (c clusterComposeConfigFactory) Object() runtime.Object {
	return &ClusterComposeConfig{}
}

func (c clusterComposeConfigFactory) List() runtime.Object {
	return &ClusterComposeConfigList{}
}

func (s *clusterComposeConfigClient) Controller() ClusterComposeConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterComposeConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterComposeConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterComposeConfigController{
		GenericController: genericController,
	}

	s.client.clusterComposeConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterComposeConfigClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ClusterComposeConfigController
}

func (s *clusterComposeConfigClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *clusterComposeConfigClient) Create(o *ClusterComposeConfig) (*ClusterComposeConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterComposeConfig), err
}

func (s *clusterComposeConfigClient) Get(name string, opts metav1.GetOptions) (*ClusterComposeConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterComposeConfig), err
}

func (s *clusterComposeConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterComposeConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterComposeConfig), err
}

func (s *clusterComposeConfigClient) Update(o *ClusterComposeConfig) (*ClusterComposeConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterComposeConfig), err
}

func (s *clusterComposeConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterComposeConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterComposeConfigClient) List(opts metav1.ListOptions) (*ClusterComposeConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterComposeConfigList), err
}

func (s *clusterComposeConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterComposeConfigClient) Patch(o *ClusterComposeConfig, data []byte, subresources ...string) (*ClusterComposeConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ClusterComposeConfig), err
}

func (s *clusterComposeConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterComposeConfigClient) AddHandler(name string, sync ClusterComposeConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterComposeConfigClient) AddLifecycle(name string, lifecycle ClusterComposeConfigLifecycle) {
	sync := NewClusterComposeConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterComposeConfigClient) AddClusterScopedHandler(name, clusterName string, sync ClusterComposeConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterComposeConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterComposeConfigLifecycle) {
	sync := NewClusterComposeConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
