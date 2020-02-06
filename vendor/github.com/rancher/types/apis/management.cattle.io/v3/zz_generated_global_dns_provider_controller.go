package v3

import (
	"context"
	"time"

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

func NewGlobalDNSProvider(namespace, name string, obj GlobalDNSProvider) *GlobalDNSProvider {
	obj.APIVersion, obj.Kind = GlobalDNSProviderGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalDNSProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalDNSProvider `json:"items"`
}

type GlobalDNSProviderHandlerFunc func(key string, obj *GlobalDNSProvider) (runtime.Object, error)

type GlobalDNSProviderChangeHandlerFunc func(obj *GlobalDNSProvider) (runtime.Object, error)

type GlobalDNSProviderLister interface {
	List(namespace string, selector labels.Selector) (ret []*GlobalDNSProvider, err error)
	Get(namespace, name string) (*GlobalDNSProvider, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GlobalDNSProviderInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*GlobalDNSProvider) (*GlobalDNSProvider, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalDNSProvider, error)
	Get(name string, opts metav1.GetOptions) (*GlobalDNSProvider, error)
	Update(*GlobalDNSProvider) (*GlobalDNSProvider, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GlobalDNSProviderList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*GlobalDNSProviderList, error)
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

func (l *globalDnsProviderLister) List(namespace string, selector labels.Selector) (ret []*GlobalDNSProvider, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GlobalDNSProvider))
	})
	return
}

func (l *globalDnsProviderLister) Get(namespace, name string) (*GlobalDNSProvider, error) {
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
			Resource: "globalDnsProvider",
		}, key)
	}
	return obj.(*GlobalDNSProvider), nil
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
		} else if v, ok := obj.(*GlobalDNSProvider); ok {
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
		} else if v, ok := obj.(*GlobalDNSProvider); ok {
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
		} else if v, ok := obj.(*GlobalDNSProvider); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*GlobalDNSProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalDnsProviderFactory struct {
}

func (c globalDnsProviderFactory) Object() runtime.Object {
	return &GlobalDNSProvider{}
}

func (c globalDnsProviderFactory) List() runtime.Object {
	return &GlobalDNSProviderList{}
}

func (s *globalDnsProviderClient) Controller() GlobalDNSProviderController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.globalDnsProviderControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GlobalDNSProviderGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &globalDnsProviderController{
		GenericController: genericController,
	}

	s.client.globalDnsProviderControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *globalDnsProviderClient) Create(o *GlobalDNSProvider) (*GlobalDNSProvider, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) Get(name string, opts metav1.GetOptions) (*GlobalDNSProvider, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalDNSProvider, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) Update(o *GlobalDNSProvider) (*GlobalDNSProvider, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GlobalDNSProvider), err
}

func (s *globalDnsProviderClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalDnsProviderClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalDnsProviderClient) List(opts metav1.ListOptions) (*GlobalDNSProviderList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GlobalDNSProviderList), err
}

func (s *globalDnsProviderClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*GlobalDNSProviderList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*GlobalDNSProviderList), err
}

func (s *globalDnsProviderClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalDnsProviderClient) Patch(o *GlobalDNSProvider, patchType types.PatchType, data []byte, subresources ...string) (*GlobalDNSProvider, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*GlobalDNSProvider), err
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
