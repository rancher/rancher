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
	GlobalDnsProviderGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalDnsProvider",
	}
	GlobalDnsProviderResource = metav1.APIResource{
		Name:         "globaldnsproviders",
		SingularName: "globaldnsprovider",
		Namespaced:   true,

		Kind: GlobalDnsProviderGroupVersionKind.Kind,
	}

	GlobalDnsProviderGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globaldnsproviders",
	}
)

func init() {
	resource.Put(GlobalDnsProviderGroupVersionResource)
}

// Deprecated: use v3.GlobalDnsProvider instead
type GlobalDnsProvider = v3.GlobalDnsProvider

func NewGlobalDnsProvider(namespace, name string, obj v3.GlobalDnsProvider) *v3.GlobalDnsProvider {
	obj.APIVersion, obj.Kind = GlobalDnsProviderGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalDnsProviderHandlerFunc func(key string, obj *v3.GlobalDnsProvider) (runtime.Object, error)

type GlobalDnsProviderChangeHandlerFunc func(obj *v3.GlobalDnsProvider) (runtime.Object, error)

type GlobalDnsProviderLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.GlobalDnsProvider, err error)
	Get(namespace, name string) (*v3.GlobalDnsProvider, error)
}

type GlobalDnsProviderController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalDnsProviderLister
	AddHandler(ctx context.Context, name string, handler GlobalDnsProviderHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDnsProviderHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalDnsProviderHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalDnsProviderHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type GlobalDnsProviderInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.GlobalDnsProvider) (*v3.GlobalDnsProvider, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalDnsProvider, error)
	Get(name string, opts metav1.GetOptions) (*v3.GlobalDnsProvider, error)
	Update(*v3.GlobalDnsProvider) (*v3.GlobalDnsProvider, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.GlobalDnsProviderList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalDnsProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalDnsProviderController
	AddHandler(ctx context.Context, name string, sync GlobalDnsProviderHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDnsProviderHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalDnsProviderLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDnsProviderLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDnsProviderHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDnsProviderHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDnsProviderLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDnsProviderLifecycle)
}

type globalDnsProviderLister struct {
	ns         string
	controller *globalDnsProviderController
}

func (l *globalDnsProviderLister) List(namespace string, selector labels.Selector) (ret []*v3.GlobalDnsProvider, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.GlobalDnsProvider))
	})
	return
}

func (l *globalDnsProviderLister) Get(namespace, name string) (*v3.GlobalDnsProvider, error) {
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
			Group:    GlobalDnsProviderGroupVersionKind.Group,
			Resource: GlobalDnsProviderGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.GlobalDnsProvider), nil
}

type globalDnsProviderController struct {
	ns string
	controller.GenericController
}

func (c *globalDnsProviderController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalDnsProviderController) Lister() GlobalDnsProviderLister {
	return &globalDnsProviderLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *globalDnsProviderController) AddHandler(ctx context.Context, name string, handler GlobalDnsProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDnsProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsProviderController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalDnsProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDnsProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsProviderController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalDnsProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDnsProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsProviderController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalDnsProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDnsProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalDnsProviderFactory struct {
}

func (c globalDnsProviderFactory) Object() runtime.Object {
	return &v3.GlobalDnsProvider{}
}

func (c globalDnsProviderFactory) List() runtime.Object {
	return &v3.GlobalDnsProviderList{}
}

func (s *globalDnsProviderClient) Controller() GlobalDnsProviderController {
	genericController := controller.NewGenericController(s.ns, GlobalDnsProviderGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(GlobalDnsProviderGroupVersionResource, GlobalDnsProviderGroupVersionKind.Kind, true))

	return &globalDnsProviderController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type globalDnsProviderClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalDnsProviderController
}

func (s *globalDnsProviderClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalDnsProviderClient) Create(o *v3.GlobalDnsProvider) (*v3.GlobalDnsProvider, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.GlobalDnsProvider), err
}

func (s *globalDnsProviderClient) Get(name string, opts metav1.GetOptions) (*v3.GlobalDnsProvider, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.GlobalDnsProvider), err
}

func (s *globalDnsProviderClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalDnsProvider, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.GlobalDnsProvider), err
}

func (s *globalDnsProviderClient) Update(o *v3.GlobalDnsProvider) (*v3.GlobalDnsProvider, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.GlobalDnsProvider), err
}

func (s *globalDnsProviderClient) UpdateStatus(o *v3.GlobalDnsProvider) (*v3.GlobalDnsProvider, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.GlobalDnsProvider), err
}

func (s *globalDnsProviderClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalDnsProviderClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalDnsProviderClient) List(opts metav1.ListOptions) (*v3.GlobalDnsProviderList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.GlobalDnsProviderList), err
}

func (s *globalDnsProviderClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalDnsProviderList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.GlobalDnsProviderList), err
}

func (s *globalDnsProviderClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalDnsProviderClient) Patch(o *v3.GlobalDnsProvider, patchType types.PatchType, data []byte, subresources ...string) (*v3.GlobalDnsProvider, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.GlobalDnsProvider), err
}

func (s *globalDnsProviderClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalDnsProviderClient) AddHandler(ctx context.Context, name string, sync GlobalDnsProviderHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsProviderClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDnsProviderHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsProviderClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalDnsProviderLifecycle) {
	sync := NewGlobalDnsProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsProviderClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDnsProviderLifecycle) {
	sync := NewGlobalDnsProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDnsProviderHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDnsProviderHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDnsProviderLifecycle) {
	sync := NewGlobalDnsProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsProviderClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDnsProviderLifecycle) {
	sync := NewGlobalDnsProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
