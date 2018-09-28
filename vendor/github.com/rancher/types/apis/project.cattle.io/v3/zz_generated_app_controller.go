package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App
}

type AppHandlerFunc func(key string, obj *App) error

type AppLister interface {
	List(namespace string, selector labels.Selector) (ret []*App, err error)
	Get(namespace, name string) (*App, error)
}

type AppController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AppLister
	AddHandler(name string, handler AppHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler AppHandlerFunc)
	Enqueue(namespace, name string)
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
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AppController
	AddHandler(name string, sync AppHandlerFunc)
	AddLifecycle(name string, lifecycle AppLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync AppHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle AppLifecycle)
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

func (c *appController) AddHandler(name string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*App))
	})
}

func (c *appController) AddClusterScopedHandler(name, cluster string, handler AppHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*App))
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

func (s *appClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *appClient) Patch(o *App, data []byte, subresources ...string) (*App, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*App), err
}

func (s *appClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *appClient) AddHandler(name string, sync AppHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *appClient) AddLifecycle(name string, lifecycle AppLifecycle) {
	sync := NewAppLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *appClient) AddClusterScopedHandler(name, clusterName string, sync AppHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *appClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle AppLifecycle) {
	sync := NewAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
