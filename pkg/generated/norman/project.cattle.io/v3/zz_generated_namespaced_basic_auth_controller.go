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

// Deprecated: use v3.NamespacedBasicAuth instead
type NamespacedBasicAuth = v3.NamespacedBasicAuth

func NewNamespacedBasicAuth(namespace, name string, obj v3.NamespacedBasicAuth) *v3.NamespacedBasicAuth {
	obj.APIVersion, obj.Kind = NamespacedBasicAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedBasicAuthHandlerFunc func(key string, obj *v3.NamespacedBasicAuth) (runtime.Object, error)

type NamespacedBasicAuthChangeHandlerFunc func(obj *v3.NamespacedBasicAuth) (runtime.Object, error)

type NamespacedBasicAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NamespacedBasicAuth, err error)
	Get(namespace, name string) (*v3.NamespacedBasicAuth, error)
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
}

type NamespacedBasicAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NamespacedBasicAuth) (*v3.NamespacedBasicAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedBasicAuth, error)
	Get(name string, opts metav1.GetOptions) (*v3.NamespacedBasicAuth, error)
	Update(*v3.NamespacedBasicAuth) (*v3.NamespacedBasicAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NamespacedBasicAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedBasicAuthList, error)
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
	ns         string
	controller *namespacedBasicAuthController
}

func (l *namespacedBasicAuthLister) List(namespace string, selector labels.Selector) (ret []*v3.NamespacedBasicAuth, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NamespacedBasicAuth))
	})
	return
}

func (l *namespacedBasicAuthLister) Get(namespace, name string) (*v3.NamespacedBasicAuth, error) {
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
			Resource: NamespacedBasicAuthGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NamespacedBasicAuth), nil
}

type namespacedBasicAuthController struct {
	ns string
	controller.GenericController
}

func (c *namespacedBasicAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedBasicAuthController) Lister() NamespacedBasicAuthLister {
	return &namespacedBasicAuthLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *namespacedBasicAuthController) AddHandler(ctx context.Context, name string, handler NamespacedBasicAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedBasicAuth); ok {
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
		} else if v, ok := obj.(*v3.NamespacedBasicAuth); ok {
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
		} else if v, ok := obj.(*v3.NamespacedBasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*v3.NamespacedBasicAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedBasicAuthFactory struct {
}

func (c namespacedBasicAuthFactory) Object() runtime.Object {
	return &v3.NamespacedBasicAuth{}
}

func (c namespacedBasicAuthFactory) List() runtime.Object {
	return &v3.NamespacedBasicAuthList{}
}

func (s *namespacedBasicAuthClient) Controller() NamespacedBasicAuthController {
	genericController := controller.NewGenericController(s.ns, NamespacedBasicAuthGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NamespacedBasicAuthGroupVersionResource, NamespacedBasicAuthGroupVersionKind.Kind, true))

	return &namespacedBasicAuthController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *namespacedBasicAuthClient) Create(o *v3.NamespacedBasicAuth) (*v3.NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Get(name string, opts metav1.GetOptions) (*v3.NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedBasicAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Update(o *v3.NamespacedBasicAuth) (*v3.NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) UpdateStatus(o *v3.NamespacedBasicAuth) (*v3.NamespacedBasicAuth, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedBasicAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedBasicAuthClient) List(opts metav1.ListOptions) (*v3.NamespacedBasicAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NamespacedBasicAuthList), err
}

func (s *namespacedBasicAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedBasicAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NamespacedBasicAuthList), err
}

func (s *namespacedBasicAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedBasicAuthClient) Patch(o *v3.NamespacedBasicAuth, patchType types.PatchType, data []byte, subresources ...string) (*v3.NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NamespacedBasicAuth), err
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
