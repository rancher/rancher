package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ServiceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Service",
	}
	ServiceResource = metav1.APIResource{
		Name:         "services",
		SingularName: "service",
		Namespaced:   true,

		Kind: ServiceGroupVersionKind.Kind,
	}
)

type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Service
}

type ServiceHandlerFunc func(key string, obj *v1.Service) error

type ServiceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Service, err error)
	Get(namespace, name string) (*v1.Service, error)
}

type ServiceController interface {
	Informer() cache.SharedIndexInformer
	Lister() ServiceLister
	AddHandler(name string, handler ServiceHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ServiceHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ServiceInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.Service) (*v1.Service, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Service, error)
	Get(name string, opts metav1.GetOptions) (*v1.Service, error)
	Update(*v1.Service) (*v1.Service, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceController
	AddHandler(name string, sync ServiceHandlerFunc)
	AddLifecycle(name string, lifecycle ServiceLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ServiceHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ServiceLifecycle)
}

type serviceLister struct {
	controller *serviceController
}

func (l *serviceLister) List(namespace string, selector labels.Selector) (ret []*v1.Service, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Service))
	})
	return
}

func (l *serviceLister) Get(namespace, name string) (*v1.Service, error) {
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
			Group:    ServiceGroupVersionKind.Group,
			Resource: "service",
		}, name)
	}
	return obj.(*v1.Service), nil
}

type serviceController struct {
	controller.GenericController
}

func (c *serviceController) Lister() ServiceLister {
	return &serviceLister{
		controller: c,
	}
}

func (c *serviceController) AddHandler(name string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Service))
	})
}

func (c *serviceController) AddClusterScopedHandler(name, cluster string, handler ServiceHandlerFunc) {
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

		return handler(key, obj.(*v1.Service))
	})
}

type serviceFactory struct {
}

func (c serviceFactory) Object() runtime.Object {
	return &v1.Service{}
}

func (c serviceFactory) List() runtime.Object {
	return &ServiceList{}
}

func (s *serviceClient) Controller() ServiceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.serviceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ServiceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &serviceController{
		GenericController: genericController,
	}

	s.client.serviceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type serviceClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ServiceController
}

func (s *serviceClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *serviceClient) Create(o *v1.Service) (*v1.Service, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Get(name string, opts metav1.GetOptions) (*v1.Service, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Service), err
}

func (s *serviceClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Service, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Update(o *v1.Service) (*v1.Service, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *serviceClient) List(opts metav1.ListOptions) (*ServiceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ServiceList), err
}

func (s *serviceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceClient) Patch(o *v1.Service, data []byte, subresources ...string) (*v1.Service, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.Service), err
}

func (s *serviceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceClient) AddHandler(name string, sync ServiceHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *serviceClient) AddLifecycle(name string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *serviceClient) AddClusterScopedHandler(name, clusterName string, sync ServiceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *serviceClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
