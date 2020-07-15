package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	GlobalDNSProviderGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalDNSProvider",
	}
	GlobalDNSProviderResource = metav1.APIResource{
		Name:         "globaldnsproviders",
		SingularName: "globaldnsprovider",
		Namespaced:   true,

		Kind: GlobalDNSProviderGroupVersionKind.Kind,
	}

	GlobalDNSProviderGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globaldnsproviders",
	}
)

func init() {
	resource.Put(GlobalDNSProviderGroupVersionResource)
}

// Deprecated use v3.GlobalDNSProvider instead
type GlobalDNSProvider = v3.GlobalDNSProvider

func NewGlobalDNSProvider(namespace, name string, obj v3.GlobalDNSProvider) *v3.GlobalDNSProvider {
	obj.APIVersion, obj.Kind = GlobalDNSProviderGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalDNSProviderHandlerFunc func(key string, obj *v3.GlobalDNSProvider) (runtime.Object, error)

type GlobalDNSProviderChangeHandlerFunc func(obj *v3.GlobalDNSProvider) (runtime.Object, error)

type GlobalDNSProviderLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.GlobalDNSProvider, err error)
	Get(namespace, name string) (*v3.GlobalDNSProvider, error)
}

type GlobalDNSProviderController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalDNSProviderLister
	AddHandler(ctx context.Context, name string, handler GlobalDNSProviderHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDNSProviderHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalDNSProviderHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalDNSProviderHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type GlobalDNSProviderInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.GlobalDNSProvider) (*v3.GlobalDNSProvider, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalDNSProvider, error)
	Get(name string, opts metav1.GetOptions) (*v3.GlobalDNSProvider, error)
	Update(*v3.GlobalDNSProvider) (*v3.GlobalDNSProvider, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.GlobalDNSProviderList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalDNSProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalDNSProviderController
	AddHandler(ctx context.Context, name string, sync GlobalDNSProviderHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDNSProviderHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalDNSProviderLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDNSProviderLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDNSProviderHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDNSProviderHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDNSProviderLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDNSProviderLifecycle)
}

type globalDnsProviderLister struct {
	controller *globalDnsProviderController
}

func (l *globalDnsProviderLister) List(namespace string, selector labels.Selector) (ret []*v3.GlobalDNSProvider, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.GlobalDNSProvider))
	})
	return
}

func (l *globalDnsProviderLister) Get(namespace, name string) (*v3.GlobalDNSProvider, error) {
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
			Group:    GlobalDNSProviderGroupVersionKind.Group,
			Resource: GlobalDNSProviderGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.GlobalDNSProvider), nil
}

type globalDnsProviderController struct {
	controller.GenericController
}

func (c *globalDnsProviderController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalDnsProviderController) Lister() GlobalDNSProviderLister {
	return &globalDnsProviderLister{
		controller: c,
	}
}

func (c *globalDnsProviderController) AddHandler(ctx context.Context, name string, handler GlobalDNSProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDNSProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsProviderController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalDNSProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDNSProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsProviderController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalDNSProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDNSProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsProviderController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalDNSProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDNSProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalDnsProviderFactory struct {
}

func (c globalDnsProviderFactory) Object() runtime.Object {
	return &v3.GlobalDNSProvider{}
}

func (c globalDnsProviderFactory) List() runtime.Object {
	return &v3.GlobalDNSProviderList{}
}

func (s *globalDnsProviderClient) Controller() GlobalDNSProviderController {
	genericController := controller.NewGenericController(GlobalDNSProviderGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(GlobalDNSProviderGroupVersionResource, GlobalDNSProviderGroupVersionKind.Kind, true))

	return &globalDnsProviderController{
		GenericController: genericController,
	}
}

type globalDnsProviderClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalDNSProviderController
}

func (s *globalDnsProviderClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalDnsProviderClient) Create(o *v3.GlobalDNSProvider) (*v3.GlobalDNSProvider, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) Get(name string, opts metav1.GetOptions) (*v3.GlobalDNSProvider, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalDNSProvider, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) Update(o *v3.GlobalDNSProvider) (*v3.GlobalDNSProvider, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) UpdateStatus(o *v3.GlobalDNSProvider) (*v3.GlobalDNSProvider, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalDnsProviderClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalDnsProviderClient) List(opts metav1.ListOptions) (*v3.GlobalDNSProviderList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.GlobalDNSProviderList), err
}

func (s *globalDnsProviderClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalDNSProviderList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.GlobalDNSProviderList), err
}

func (s *globalDnsProviderClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalDnsProviderClient) Patch(o *v3.GlobalDNSProvider, patchType types.PatchType, data []byte, subresources ...string) (*v3.GlobalDNSProvider, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalDnsProviderClient) AddHandler(ctx context.Context, name string, sync GlobalDNSProviderHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsProviderClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDNSProviderHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsProviderClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalDNSProviderLifecycle) {
	sync := NewGlobalDNSProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsProviderClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDNSProviderLifecycle) {
	sync := NewGlobalDNSProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDNSProviderHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDNSProviderHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDNSProviderLifecycle) {
	sync := NewGlobalDNSProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDNSProviderLifecycle) {
	sync := NewGlobalDNSProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
