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
	MultiClusterAppGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "MultiClusterApp",
	}
	MultiClusterAppResource = metav1.APIResource{
		Name:         "multiclusterapps",
		SingularName: "multiclusterapp",
		Namespaced:   true,

		Kind: MultiClusterAppGroupVersionKind.Kind,
	}

	MultiClusterAppGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "multiclusterapps",
	}
)

func init() {
	resource.Put(MultiClusterAppGroupVersionResource)
}

// Deprecated: use v3.MultiClusterApp instead
type MultiClusterApp = v3.MultiClusterApp

func NewMultiClusterApp(namespace, name string, obj v3.MultiClusterApp) *v3.MultiClusterApp {
	obj.APIVersion, obj.Kind = MultiClusterAppGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type MultiClusterAppHandlerFunc func(key string, obj *v3.MultiClusterApp) (runtime.Object, error)

type MultiClusterAppChangeHandlerFunc func(obj *v3.MultiClusterApp) (runtime.Object, error)

type MultiClusterAppLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.MultiClusterApp, err error)
	Get(namespace, name string) (*v3.MultiClusterApp, error)
}

type MultiClusterAppController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() MultiClusterAppLister
	AddHandler(ctx context.Context, name string, handler MultiClusterAppHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler MultiClusterAppHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler MultiClusterAppHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type MultiClusterAppInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.MultiClusterApp) (*v3.MultiClusterApp, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.MultiClusterApp, error)
	Get(name string, opts metav1.GetOptions) (*v3.MultiClusterApp, error)
	Update(*v3.MultiClusterApp) (*v3.MultiClusterApp, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.MultiClusterAppList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.MultiClusterAppList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MultiClusterAppController
	AddHandler(ctx context.Context, name string, sync MultiClusterAppHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppLifecycle)
}

type multiClusterAppLister struct {
	ns         string
	controller *multiClusterAppController
}

func (l *multiClusterAppLister) List(namespace string, selector labels.Selector) (ret []*v3.MultiClusterApp, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.MultiClusterApp))
	})
	return
}

func (l *multiClusterAppLister) Get(namespace, name string) (*v3.MultiClusterApp, error) {
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
			Group:    MultiClusterAppGroupVersionKind.Group,
			Resource: MultiClusterAppGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.MultiClusterApp), nil
}

type multiClusterAppController struct {
	ns string
	controller.GenericController
}

func (c *multiClusterAppController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *multiClusterAppController) Lister() MultiClusterAppLister {
	return &multiClusterAppLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *multiClusterAppController) AddHandler(ctx context.Context, name string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterApp); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterApp); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterApp); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterApp); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type multiClusterAppFactory struct {
}

func (c multiClusterAppFactory) Object() runtime.Object {
	return &v3.MultiClusterApp{}
}

func (c multiClusterAppFactory) List() runtime.Object {
	return &v3.MultiClusterAppList{}
}

func (s *multiClusterAppClient) Controller() MultiClusterAppController {
	genericController := controller.NewGenericController(s.ns, MultiClusterAppGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(MultiClusterAppGroupVersionResource, MultiClusterAppGroupVersionKind.Kind, true))

	return &multiClusterAppController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type multiClusterAppClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   MultiClusterAppController
}

func (s *multiClusterAppClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *multiClusterAppClient) Create(o *v3.MultiClusterApp) (*v3.MultiClusterApp, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.MultiClusterApp), err
}

func (s *multiClusterAppClient) Get(name string, opts metav1.GetOptions) (*v3.MultiClusterApp, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.MultiClusterApp), err
}

func (s *multiClusterAppClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.MultiClusterApp, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.MultiClusterApp), err
}

func (s *multiClusterAppClient) Update(o *v3.MultiClusterApp) (*v3.MultiClusterApp, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.MultiClusterApp), err
}

func (s *multiClusterAppClient) UpdateStatus(o *v3.MultiClusterApp) (*v3.MultiClusterApp, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.MultiClusterApp), err
}

func (s *multiClusterAppClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *multiClusterAppClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *multiClusterAppClient) List(opts metav1.ListOptions) (*v3.MultiClusterAppList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.MultiClusterAppList), err
}

func (s *multiClusterAppClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.MultiClusterAppList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.MultiClusterAppList), err
}

func (s *multiClusterAppClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *multiClusterAppClient) Patch(o *v3.MultiClusterApp, patchType types.PatchType, data []byte, subresources ...string) (*v3.MultiClusterApp, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.MultiClusterApp), err
}

func (s *multiClusterAppClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *multiClusterAppClient) AddHandler(ctx context.Context, name string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppClient) AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *multiClusterAppClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
