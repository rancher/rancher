package v1

import (
	"context"
	"time"

	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
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
	ServiceMonitorGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ServiceMonitor",
	}
	ServiceMonitorResource = metav1.APIResource{
		Name:         "servicemonitors",
		SingularName: "servicemonitor",
		Namespaced:   true,

		Kind: ServiceMonitorGroupVersionKind.Kind,
	}

	ServiceMonitorGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "servicemonitors",
	}
)

func init() {
	resource.Put(ServiceMonitorGroupVersionResource)
}

// Deprecated: use v1.ServiceMonitor instead
type ServiceMonitor = v1.ServiceMonitor

func NewServiceMonitor(namespace, name string, obj v1.ServiceMonitor) *v1.ServiceMonitor {
	obj.APIVersion, obj.Kind = ServiceMonitorGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceMonitorHandlerFunc func(key string, obj *v1.ServiceMonitor) (runtime.Object, error)

type ServiceMonitorChangeHandlerFunc func(obj *v1.ServiceMonitor) (runtime.Object, error)

type ServiceMonitorLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ServiceMonitor, err error)
	Get(namespace, name string) (*v1.ServiceMonitor, error)
}

type ServiceMonitorController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceMonitorLister
	AddHandler(ctx context.Context, name string, handler ServiceMonitorHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceMonitorHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceMonitorHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ServiceMonitorHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ServiceMonitorInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ServiceMonitor) (*v1.ServiceMonitor, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error)
	Get(name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error)
	Update(*v1.ServiceMonitor) (*v1.ServiceMonitor, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ServiceMonitorList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ServiceMonitorList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceMonitorController
	AddHandler(ctx context.Context, name string, sync ServiceMonitorHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceMonitorHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceMonitorLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceMonitorLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceMonitorHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceMonitorHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceMonitorLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceMonitorLifecycle)
}

type serviceMonitorLister struct {
	ns         string
	controller *serviceMonitorController
}

func (l *serviceMonitorLister) List(namespace string, selector labels.Selector) (ret []*v1.ServiceMonitor, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ServiceMonitor))
	})
	return
}

func (l *serviceMonitorLister) Get(namespace, name string) (*v1.ServiceMonitor, error) {
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
			Group:    ServiceMonitorGroupVersionKind.Group,
			Resource: ServiceMonitorGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ServiceMonitor), nil
}

type serviceMonitorController struct {
	ns string
	controller.GenericController
}

func (c *serviceMonitorController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceMonitorController) Lister() ServiceMonitorLister {
	return &serviceMonitorLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *serviceMonitorController) AddHandler(ctx context.Context, name string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceMonitorController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceMonitorController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceMonitorController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceMonitorFactory struct {
}

func (c serviceMonitorFactory) Object() runtime.Object {
	return &v1.ServiceMonitor{}
}

func (c serviceMonitorFactory) List() runtime.Object {
	return &v1.ServiceMonitorList{}
}

func (s *serviceMonitorClient) Controller() ServiceMonitorController {
	genericController := controller.NewGenericController(s.ns, ServiceMonitorGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ServiceMonitorGroupVersionResource, ServiceMonitorGroupVersionKind.Kind, true))

	return &serviceMonitorController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type serviceMonitorClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceMonitorController
}

func (s *serviceMonitorClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *serviceMonitorClient) Create(o *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) Get(name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) Update(o *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) UpdateStatus(o *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceMonitorClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceMonitorClient) List(opts metav1.ListOptions) (*v1.ServiceMonitorList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ServiceMonitorList), err
}

func (s *serviceMonitorClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ServiceMonitorList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ServiceMonitorList), err
}

func (s *serviceMonitorClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceMonitorClient) Patch(o *v1.ServiceMonitor, patchType types.PatchType, data []byte, subresources ...string) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceMonitorClient) AddHandler(ctx context.Context, name string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceMonitorClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceMonitorClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceMonitorClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceMonitorClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceMonitorClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *serviceMonitorClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceMonitorClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
