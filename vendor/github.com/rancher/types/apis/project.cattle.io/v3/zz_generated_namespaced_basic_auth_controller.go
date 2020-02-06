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
	NamespacedBasicAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedBasicAuth",
	}
	NamespacedBasicAuthResource = metav1.APIResource{
		Name:         "namespacedbasicauths",
		SingularName: "namespacedbasicauth",
		Namespaced:   true,

		Kind: NamespacedBasicAuthGroupVersionKind.Kind,
	}

	NamespacedBasicAuthGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespacedbasicauths",
	}
)

func init() {
	resource.Put(NamespacedBasicAuthGroupVersionResource)
}

func NewNamespacedBasicAuth(namespace, name string, obj NamespacedBasicAuth) *NamespacedBasicAuth {
	obj.APIVersion, obj.Kind = NamespacedBasicAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedBasicAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedBasicAuth `json:"items"`
}

type NamespacedBasicAuthHandlerFunc func(key string, obj *NamespacedBasicAuth) (runtime.Object, error)

type NamespacedBasicAuthChangeHandlerFunc func(obj *NamespacedBasicAuth) (runtime.Object, error)

type NamespacedBasicAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedBasicAuth, err error)
	Get(namespace, name string) (*NamespacedBasicAuth, error)
}

type NamespacedBasicAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedBasicAuthLister
	AddHandler(ctx context.Context, name string, handler NamespacedBasicAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedBasicAuthHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedBasicAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedBasicAuthHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedBasicAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error)
	Update(*NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedBasicAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*NamespacedBasicAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedBasicAuthController
	AddHandler(ctx context.Context, name string, sync NamespacedBasicAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedBasicAuthHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedBasicAuthLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedBasicAuthLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedBasicAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedBasicAuthHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedBasicAuthLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedBasicAuthLifecycle)
}

type namespacedBasicAuthLister struct {
	controller *namespacedBasicAuthController
}

func (l *namespacedBasicAuthLister) List(namespace string, selector labels.Selector) (ret []*NamespacedBasicAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedBasicAuth))
	})
	return
}

func (l *namespacedBasicAuthLister) Get(namespace, name string) (*NamespacedBasicAuth, error) {
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
			Group:    NamespacedBasicAuthGroupVersionKind.Group,
			Resource: "namespacedBasicAuth",
		}, key)
	}
	return obj.(*NamespacedBasicAuth), nil
}

type namespacedBasicAuthController struct {
	controller.GenericController
}

func (c *namespacedBasicAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedBasicAuthController) Lister() NamespacedBasicAuthLister {
	return &namespacedBasicAuthLister{
		controller: c,
	}
}

func (c *namespacedBasicAuthController) AddHandler(ctx context.Context, name string, handler NamespacedBasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedBasicAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedBasicAuthController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedBasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedBasicAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedBasicAuthController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedBasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedBasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedBasicAuthController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedBasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedBasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedBasicAuthFactory struct {
}

func (c namespacedBasicAuthFactory) Object() runtime.Object {
	return &NamespacedBasicAuth{}
}

func (c namespacedBasicAuthFactory) List() runtime.Object {
	return &NamespacedBasicAuthList{}
}

func (s *namespacedBasicAuthClient) Controller() NamespacedBasicAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedBasicAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedBasicAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedBasicAuthController{
		GenericController: genericController,
	}

	s.client.namespacedBasicAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedBasicAuthClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedBasicAuthController
}

func (s *namespacedBasicAuthClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedBasicAuthClient) Create(o *NamespacedBasicAuth) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Get(name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Update(o *NamespacedBasicAuth) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedBasicAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedBasicAuthClient) List(opts metav1.ListOptions) (*NamespacedBasicAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedBasicAuthList), err
}

func (s *namespacedBasicAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*NamespacedBasicAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*NamespacedBasicAuthList), err
}

func (s *namespacedBasicAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedBasicAuthClient) Patch(o *NamespacedBasicAuth, patchType types.PatchType, data []byte, subresources ...string) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedBasicAuthClient) AddHandler(ctx context.Context, name string, sync NamespacedBasicAuthHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedBasicAuthClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedBasicAuthHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedBasicAuthClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedBasicAuthLifecycle) {
	sync := NewNamespacedBasicAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedBasicAuthClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedBasicAuthLifecycle) {
	sync := NewNamespacedBasicAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedBasicAuthClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedBasicAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedBasicAuthClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedBasicAuthHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedBasicAuthClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedBasicAuthLifecycle) {
	sync := NewNamespacedBasicAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedBasicAuthClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedBasicAuthLifecycle) {
	sync := NewNamespacedBasicAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
