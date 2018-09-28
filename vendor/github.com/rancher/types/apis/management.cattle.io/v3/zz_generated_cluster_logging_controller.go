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
	ClusterLoggingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterLogging",
	}
	ClusterLoggingResource = metav1.APIResource{
		Name:         "clusterloggings",
		SingularName: "clusterlogging",
		Namespaced:   true,

		Kind: ClusterLoggingGroupVersionKind.Kind,
	}
)

type ClusterLoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterLogging
}

type ClusterLoggingHandlerFunc func(key string, obj *ClusterLogging) error

type ClusterLoggingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterLogging, err error)
	Get(namespace, name string) (*ClusterLogging, error)
}

type ClusterLoggingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterLoggingLister
	AddHandler(name string, handler ClusterLoggingHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterLoggingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterLoggingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterLogging) (*ClusterLogging, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterLogging, error)
	Get(name string, opts metav1.GetOptions) (*ClusterLogging, error)
	Update(*ClusterLogging) (*ClusterLogging, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterLoggingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterLoggingController
	AddHandler(name string, sync ClusterLoggingHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterLoggingLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterLoggingHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterLoggingLifecycle)
}

type clusterLoggingLister struct {
	controller *clusterLoggingController
}

func (l *clusterLoggingLister) List(namespace string, selector labels.Selector) (ret []*ClusterLogging, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterLogging))
	})
	return
}

func (l *clusterLoggingLister) Get(namespace, name string) (*ClusterLogging, error) {
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
			Group:    ClusterLoggingGroupVersionKind.Group,
			Resource: "clusterLogging",
		}, key)
	}
	return obj.(*ClusterLogging), nil
}

type clusterLoggingController struct {
	controller.GenericController
}

func (c *clusterLoggingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterLoggingController) Lister() ClusterLoggingLister {
	return &clusterLoggingLister{
		controller: c,
	}
}

func (c *clusterLoggingController) AddHandler(name string, handler ClusterLoggingHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ClusterLogging))
	})
}

func (c *clusterLoggingController) AddClusterScopedHandler(name, cluster string, handler ClusterLoggingHandlerFunc) {
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

		return handler(key, obj.(*ClusterLogging))
	})
}

type clusterLoggingFactory struct {
}

func (c clusterLoggingFactory) Object() runtime.Object {
	return &ClusterLogging{}
}

func (c clusterLoggingFactory) List() runtime.Object {
	return &ClusterLoggingList{}
}

func (s *clusterLoggingClient) Controller() ClusterLoggingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterLoggingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterLoggingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterLoggingController{
		GenericController: genericController,
	}

	s.client.clusterLoggingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterLoggingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterLoggingController
}

func (s *clusterLoggingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterLoggingClient) Create(o *ClusterLogging) (*ClusterLogging, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) Get(name string, opts metav1.GetOptions) (*ClusterLogging, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterLogging, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) Update(o *ClusterLogging) (*ClusterLogging, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterLoggingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterLoggingClient) List(opts metav1.ListOptions) (*ClusterLoggingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterLoggingList), err
}

func (s *clusterLoggingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterLoggingClient) Patch(o *ClusterLogging, data []byte, subresources ...string) (*ClusterLogging, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterLoggingClient) AddHandler(name string, sync ClusterLoggingHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterLoggingClient) AddLifecycle(name string, lifecycle ClusterLoggingLifecycle) {
	sync := NewClusterLoggingLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterLoggingClient) AddClusterScopedHandler(name, clusterName string, sync ClusterLoggingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterLoggingClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterLoggingLifecycle) {
	sync := NewClusterLoggingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
