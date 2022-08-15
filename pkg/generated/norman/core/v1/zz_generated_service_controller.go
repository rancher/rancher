package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

	ServiceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "services",
	}
)

func init() {
	resource.Put(ServiceGroupVersionResource)
}

// Deprecated: use v1.Service instead
type Service = v1.Service

func NewService(namespace, name string, obj v1.Service) *v1.Service {
	obj.APIVersion, obj.Kind = ServiceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceHandlerFunc func(key string, obj *v1.Service) (runtime.Object, error)

type ServiceChangeHandlerFunc func(obj *v1.Service) (runtime.Object, error)

type ServiceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Service, err error)
	Get(namespace, name string) (*v1.Service, error)
}

type ServiceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceLister
	AddHandler(ctx context.Context, name string, handler ServiceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ServiceHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ServiceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Service) (*v1.Service, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Service, error)
	Get(name string, opts metav1.GetOptions) (*v1.Service, error)
	Update(*v1.Service) (*v1.Service, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ServiceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceController
	AddHandler(ctx context.Context, name string, sync ServiceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceLifecycle)
}

type serviceLister struct {
	ns         string
	controller *serviceController
}

func (l *serviceLister) List(namespace string, selector labels.Selector) (ret []*v1.Service, err error) {
	if namespace == "" {
		namespace = l.ns
	}
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
			Resource: ServiceGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Service), nil
}

type serviceController struct {
	ns string
	controller.GenericController
}

func (c *serviceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceController) Lister() ServiceLister {
	return &serviceLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *serviceController) AddHandler(ctx context.Context, name string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Service); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Service); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Service); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Service); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceFactory struct {
}

func (c serviceFactory) Object() runtime.Object {
	return &v1.Service{}
}

func (c serviceFactory) List() runtime.Object {
	return &v1.ServiceList{}
}

func (s *serviceClient) Controller() ServiceController {
	genericController := controller.NewGenericController(s.ns, ServiceGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ServiceGroupVersionResource, ServiceGroupVersionKind.Kind, true))

	return &serviceController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type serviceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceController
}

func (s *serviceClient) ObjectClient() *objectclient.ObjectClient {
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

func (s *serviceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Service, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Update(o *v1.Service) (*v1.Service, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Service), err
}

func (s *serviceClient) UpdateStatus(o *v1.Service) (*v1.Service, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Service), err
}

func (s *serviceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceClient) List(opts metav1.ListOptions) (*v1.ServiceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ServiceList), err
}

func (s *serviceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ServiceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ServiceList), err
}

func (s *serviceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceClient) Patch(o *v1.Service, patchType types.PatchType, data []byte, subresources ...string) (*v1.Service, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Service), err
}

func (s *serviceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceClient) AddHandler(ctx context.Context, name string, sync ServiceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *serviceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceLifecycle) {
	sync := NewServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
