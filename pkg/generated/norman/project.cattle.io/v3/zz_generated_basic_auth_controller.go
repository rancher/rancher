package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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

// Deprecated: use v3.BasicAuth instead
type BasicAuth = v3.BasicAuth

func NewBasicAuth(namespace, name string, obj v3.BasicAuth) *v3.BasicAuth {
	obj.APIVersion, obj.Kind = BasicAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type BasicAuthHandlerFunc func(key string, obj *v3.BasicAuth) (runtime.Object, error)

type BasicAuthChangeHandlerFunc func(obj *v3.BasicAuth) (runtime.Object, error)

type BasicAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.BasicAuth, err error)
	Get(namespace, name string) (*v3.BasicAuth, error)
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
}

type BasicAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.BasicAuth) (*v3.BasicAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.BasicAuth, error)
	Get(name string, opts metav1.GetOptions) (*v3.BasicAuth, error)
	Update(*v3.BasicAuth) (*v3.BasicAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.BasicAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.BasicAuthList, error)
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
	ns         string
	controller *basicAuthController
}

func (l *basicAuthLister) List(namespace string, selector labels.Selector) (ret []*v3.BasicAuth, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.BasicAuth))
	})
	return
}

func (l *basicAuthLister) Get(namespace, name string) (*v3.BasicAuth, error) {
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
			Resource: BasicAuthGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.BasicAuth), nil
}

type basicAuthController struct {
	ns string
	controller.GenericController
}

func (c *basicAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *basicAuthController) Lister() BasicAuthLister {
	return &basicAuthLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *basicAuthController) AddHandler(ctx context.Context, name string, handler BasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.BasicAuth); ok {
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
		} else if v, ok := obj.(*v3.BasicAuth); ok {
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
		} else if v, ok := obj.(*v3.BasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*v3.BasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type basicAuthFactory struct {
}

func (c basicAuthFactory) Object() runtime.Object {
	return &v3.BasicAuth{}
}

func (c basicAuthFactory) List() runtime.Object {
	return &v3.BasicAuthList{}
}

func (s *basicAuthClient) Controller() BasicAuthController {
	genericController := controller.NewGenericController(s.ns, BasicAuthGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(BasicAuthGroupVersionResource, BasicAuthGroupVersionKind.Kind, true))

	return &basicAuthController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *basicAuthClient) Create(o *v3.BasicAuth) (*v3.BasicAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.BasicAuth), err
}

func (s *basicAuthClient) Get(name string, opts metav1.GetOptions) (*v3.BasicAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.BasicAuth), err
}

func (s *basicAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.BasicAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.BasicAuth), err
}

func (s *basicAuthClient) Update(o *v3.BasicAuth) (*v3.BasicAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.BasicAuth), err
}

func (s *basicAuthClient) UpdateStatus(o *v3.BasicAuth) (*v3.BasicAuth, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.BasicAuth), err
}

func (s *basicAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *basicAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *basicAuthClient) List(opts metav1.ListOptions) (*v3.BasicAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.BasicAuthList), err
}

func (s *basicAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.BasicAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.BasicAuthList), err
}

func (s *basicAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *basicAuthClient) Patch(o *v3.BasicAuth, patchType types.PatchType, data []byte, subresources ...string) (*v3.BasicAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.BasicAuth), err
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
