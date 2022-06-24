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
	AppGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "App",
	}
	AppResource = metav1.APIResource{
		Name:         "apps",
		SingularName: "app",
		Namespaced:   true,

		Kind: AppGroupVersionKind.Kind,
	}

	AppGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "apps",
	}
)

func init() {
	resource.Put(AppGroupVersionResource)
}

// Deprecated: use v3.App instead
type App = v3.App

func NewApp(namespace, name string, obj v3.App) *v3.App {
	obj.APIVersion, obj.Kind = AppGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AppHandlerFunc func(key string, obj *v3.App) (runtime.Object, error)

type AppChangeHandlerFunc func(obj *v3.App) (runtime.Object, error)

type AppLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.App, err error)
	Get(namespace, name string) (*v3.App, error)
}

type AppController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AppLister
	AddHandler(ctx context.Context, name string, handler AppHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AppHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AppHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler AppHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type AppInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.App) (*v3.App, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.App, error)
	Get(name string, opts metav1.GetOptions) (*v3.App, error)
	Update(*v3.App) (*v3.App, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.AppList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.AppList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AppController
	AddHandler(ctx context.Context, name string, sync AppHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AppHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AppLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AppLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AppHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AppHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AppLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AppLifecycle)
}

type appLister struct {
	ns         string
	controller *appController
}

func (l *appLister) List(namespace string, selector labels.Selector) (ret []*v3.App, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.App))
	})
	return
}

func (l *appLister) Get(namespace, name string) (*v3.App, error) {
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
			Group:    AppGroupVersionKind.Group,
			Resource: AppGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.App), nil
}

type appController struct {
	ns string
	controller.GenericController
}

func (c *appController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *appController) Lister() AppLister {
	return &appLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *appController) AddHandler(ctx context.Context, name string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.App); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *appController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.App); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *appController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.App); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *appController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.App); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type appFactory struct {
}

func (c appFactory) Object() runtime.Object {
	return &v3.App{}
}

func (c appFactory) List() runtime.Object {
	return &v3.AppList{}
}

func (s *appClient) Controller() AppController {
	genericController := controller.NewGenericController(s.ns, AppGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(AppGroupVersionResource, AppGroupVersionKind.Kind, true))

	return &appController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type appClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AppController
}

func (s *appClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *appClient) Create(o *v3.App) (*v3.App, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.App), err
}

func (s *appClient) Get(name string, opts metav1.GetOptions) (*v3.App, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.App), err
}

func (s *appClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.App, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.App), err
}

func (s *appClient) Update(o *v3.App) (*v3.App, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.App), err
}

func (s *appClient) UpdateStatus(o *v3.App) (*v3.App, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.App), err
}

func (s *appClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *appClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *appClient) List(opts metav1.ListOptions) (*v3.AppList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.AppList), err
}

func (s *appClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.AppList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.AppList), err
}

func (s *appClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *appClient) Patch(o *v3.App, patchType types.PatchType, data []byte, subresources ...string) (*v3.App, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.App), err
}

func (s *appClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *appClient) AddHandler(ctx context.Context, name string, sync AppHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *appClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AppHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *appClient) AddLifecycle(ctx context.Context, name string, lifecycle AppLifecycle) {
	sync := NewAppLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *appClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AppLifecycle) {
	sync := NewAppLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *appClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AppHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *appClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AppHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *appClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AppLifecycle) {
	sync := NewAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *appClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AppLifecycle) {
	sync := NewAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
