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
	NamespaceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Namespace",
	}
	NamespaceResource = metav1.APIResource{
		Name:         "namespaces",
		SingularName: "namespace",
		Namespaced:   false,
		Kind:         NamespaceGroupVersionKind.Kind,
	}

	NamespaceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespaces",
	}
)

func init() {
	resource.Put(NamespaceGroupVersionResource)
}

// Deprecated: use v1.Namespace instead
type Namespace = v1.Namespace

func NewNamespace(namespace, name string, obj v1.Namespace) *v1.Namespace {
	obj.APIVersion, obj.Kind = NamespaceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespaceHandlerFunc func(key string, obj *v1.Namespace) (runtime.Object, error)

type NamespaceChangeHandlerFunc func(obj *v1.Namespace) (runtime.Object, error)

type NamespaceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Namespace, err error)
	Get(namespace, name string) (*v1.Namespace, error)
}

type NamespaceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespaceLister
	AddHandler(ctx context.Context, name string, handler NamespaceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespaceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespaceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespaceHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NamespaceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Namespace) (*v1.Namespace, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Namespace, error)
	Get(name string, opts metav1.GetOptions) (*v1.Namespace, error)
	Update(*v1.Namespace) (*v1.Namespace, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.NamespaceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.NamespaceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespaceController
	AddHandler(ctx context.Context, name string, sync NamespaceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespaceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespaceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespaceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespaceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespaceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespaceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespaceLifecycle)
}

type namespaceLister struct {
	ns         string
	controller *namespaceController
}

func (l *namespaceLister) List(namespace string, selector labels.Selector) (ret []*v1.Namespace, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Namespace))
	})
	return
}

func (l *namespaceLister) Get(namespace, name string) (*v1.Namespace, error) {
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
			Group:    NamespaceGroupVersionKind.Group,
			Resource: NamespaceGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Namespace), nil
}

type namespaceController struct {
	ns string
	controller.GenericController
}

func (c *namespaceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespaceController) Lister() NamespaceLister {
	return &namespaceLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *namespaceController) AddHandler(ctx context.Context, name string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespaceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespaceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespaceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespaceFactory struct {
}

func (c namespaceFactory) Object() runtime.Object {
	return &v1.Namespace{}
}

func (c namespaceFactory) List() runtime.Object {
	return &v1.NamespaceList{}
}

func (s *namespaceClient) Controller() NamespaceController {
	genericController := controller.NewGenericController(s.ns, NamespaceGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NamespaceGroupVersionResource, NamespaceGroupVersionKind.Kind, false))

	return &namespaceController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type namespaceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespaceController
}

func (s *namespaceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespaceClient) Create(o *v1.Namespace) (*v1.Namespace, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Get(name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Update(o *v1.Namespace) (*v1.Namespace, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) UpdateStatus(o *v1.Namespace) (*v1.Namespace, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespaceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespaceClient) List(opts metav1.ListOptions) (*v1.NamespaceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.NamespaceList), err
}

func (s *namespaceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.NamespaceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.NamespaceList), err
}

func (s *namespaceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespaceClient) Patch(o *v1.Namespace, patchType types.PatchType, data []byte, subresources ...string) (*v1.Namespace, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespaceClient) AddHandler(ctx context.Context, name string, sync NamespaceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespaceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespaceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespaceClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespaceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespaceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespaceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespaceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
