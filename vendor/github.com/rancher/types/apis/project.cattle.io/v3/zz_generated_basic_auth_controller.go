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
	BasicAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "BasicAuth",
	}
	BasicAuthResource = metav1.APIResource{
		Name:         "basicauths",
		SingularName: "basicauth",
		Namespaced:   true,

		Kind: BasicAuthGroupVersionKind.Kind,
	}

	BasicAuthGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "basicauths",
	}
)

func init() {
	resource.Put(BasicAuthGroupVersionResource)
}

func NewBasicAuth(namespace, name string, obj BasicAuth) *BasicAuth {
	obj.APIVersion, obj.Kind = BasicAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type BasicAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BasicAuth `json:"items"`
}

type BasicAuthHandlerFunc func(key string, obj *BasicAuth) (runtime.Object, error)

type BasicAuthChangeHandlerFunc func(obj *BasicAuth) (runtime.Object, error)

type BasicAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*BasicAuth, err error)
	Get(namespace, name string) (*BasicAuth, error)
}

type BasicAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() BasicAuthLister
	AddHandler(ctx context.Context, name string, handler BasicAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync BasicAuthHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler BasicAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler BasicAuthHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type BasicAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*BasicAuth) (*BasicAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*BasicAuth, error)
	Get(name string, opts metav1.GetOptions) (*BasicAuth, error)
	Update(*BasicAuth) (*BasicAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*BasicAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*BasicAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() BasicAuthController
	AddHandler(ctx context.Context, name string, sync BasicAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync BasicAuthHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle BasicAuthLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle BasicAuthLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync BasicAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync BasicAuthHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle BasicAuthLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle BasicAuthLifecycle)
}

type basicAuthLister struct {
	controller *basicAuthController
}

func (l *basicAuthLister) List(namespace string, selector labels.Selector) (ret []*BasicAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*BasicAuth))
	})
	return
}

func (l *basicAuthLister) Get(namespace, name string) (*BasicAuth, error) {
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
			Group:    BasicAuthGroupVersionKind.Group,
			Resource: "basicAuth",
		}, key)
	}
	return obj.(*BasicAuth), nil
}

type basicAuthController struct {
	controller.GenericController
}

func (c *basicAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *basicAuthController) Lister() BasicAuthLister {
	return &basicAuthLister{
		controller: c,
	}
}

func (c *basicAuthController) AddHandler(ctx context.Context, name string, handler BasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*BasicAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *basicAuthController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler BasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*BasicAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *basicAuthController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler BasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*BasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *basicAuthController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler BasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*BasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type basicAuthFactory struct {
}

func (c basicAuthFactory) Object() runtime.Object {
	return &BasicAuth{}
}

func (c basicAuthFactory) List() runtime.Object {
	return &BasicAuthList{}
}

func (s *basicAuthClient) Controller() BasicAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.basicAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(BasicAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &basicAuthController{
		GenericController: genericController,
	}

	s.client.basicAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type basicAuthClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   BasicAuthController
}

func (s *basicAuthClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *basicAuthClient) Create(o *BasicAuth) (*BasicAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*BasicAuth), err
}

func (s *basicAuthClient) Get(name string, opts metav1.GetOptions) (*BasicAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*BasicAuth), err
}

func (s *basicAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*BasicAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*BasicAuth), err
}

func (s *basicAuthClient) Update(o *BasicAuth) (*BasicAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*BasicAuth), err
}

func (s *basicAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *basicAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *basicAuthClient) List(opts metav1.ListOptions) (*BasicAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*BasicAuthList), err
}

func (s *basicAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*BasicAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*BasicAuthList), err
}

func (s *basicAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *basicAuthClient) Patch(o *BasicAuth, patchType types.PatchType, data []byte, subresources ...string) (*BasicAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*BasicAuth), err
}

func (s *basicAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *basicAuthClient) AddHandler(ctx context.Context, name string, sync BasicAuthHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *basicAuthClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync BasicAuthHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *basicAuthClient) AddLifecycle(ctx context.Context, name string, lifecycle BasicAuthLifecycle) {
	sync := NewBasicAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *basicAuthClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle BasicAuthLifecycle) {
	sync := NewBasicAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *basicAuthClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync BasicAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *basicAuthClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync BasicAuthHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *basicAuthClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle BasicAuthLifecycle) {
	sync := NewBasicAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *basicAuthClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle BasicAuthLifecycle) {
	sync := NewBasicAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
