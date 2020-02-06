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

func NewApp(namespace, name string, obj App) *App {
	obj.APIVersion, obj.Kind = AppGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

type AppHandlerFunc func(key string, obj *App) (runtime.Object, error)

type AppChangeHandlerFunc func(obj *App) (runtime.Object, error)

type AppLister interface {
	List(namespace string, selector labels.Selector) (ret []*App, err error)
	Get(namespace, name string) (*App, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type AppInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*App) (*App, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*App, error)
	Get(name string, opts metav1.GetOptions) (*App, error)
	Update(*App) (*App, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*AppList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*AppList, error)
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
	controller *appController
}

func (l *appLister) List(namespace string, selector labels.Selector) (ret []*App, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*App))
	})
	return
}

func (l *appLister) Get(namespace, name string) (*App, error) {
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
			Resource: "app",
		}, key)
	}
	return obj.(*App), nil
}

type appController struct {
	controller.GenericController
}

func (c *appController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *appController) Lister() AppLister {
	return &appLister{
		controller: c,
	}
}

func (c *appController) AddHandler(ctx context.Context, name string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*App); ok {
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
		} else if v, ok := obj.(*App); ok {
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
		} else if v, ok := obj.(*App); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*App); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type appFactory struct {
}

func (c appFactory) Object() runtime.Object {
	return &App{}
}

func (c appFactory) List() runtime.Object {
	return &AppList{}
}

func (s *appClient) Controller() AppController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.appControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(AppGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &appController{
		GenericController: genericController,
	}

	s.client.appControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *appClient) Create(o *App) (*App, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*App), err
}

func (s *appClient) Get(name string, opts metav1.GetOptions) (*App, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*App), err
}

func (s *appClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*App, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*App), err
}

func (s *appClient) Update(o *App) (*App, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*App), err
}

func (s *appClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *appClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *appClient) List(opts metav1.ListOptions) (*AppList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*AppList), err
}

func (s *appClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*AppList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*AppList), err
}

func (s *appClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *appClient) Patch(o *App, patchType types.PatchType, data []byte, subresources ...string) (*App, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*App), err
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
