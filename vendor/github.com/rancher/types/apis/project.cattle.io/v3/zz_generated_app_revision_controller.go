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
	AppRevisionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "AppRevision",
	}
	AppRevisionResource = metav1.APIResource{
		Name:         "apprevisions",
		SingularName: "apprevision",
		Namespaced:   true,

		Kind: AppRevisionGroupVersionKind.Kind,
	}
)

type AppRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppRevision
}

type AppRevisionHandlerFunc func(key string, obj *AppRevision) error

type AppRevisionLister interface {
	List(namespace string, selector labels.Selector) (ret []*AppRevision, err error)
	Get(namespace, name string) (*AppRevision, error)
}

type AppRevisionController interface {
	Informer() cache.SharedIndexInformer
	Lister() AppRevisionLister
	AddHandler(name string, handler AppRevisionHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler AppRevisionHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type AppRevisionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*AppRevision) (*AppRevision, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*AppRevision, error)
	Get(name string, opts metav1.GetOptions) (*AppRevision, error)
	Update(*AppRevision) (*AppRevision, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*AppRevisionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AppRevisionController
	AddHandler(name string, sync AppRevisionHandlerFunc)
	AddLifecycle(name string, lifecycle AppRevisionLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync AppRevisionHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle AppRevisionLifecycle)
}

type appRevisionLister struct {
	controller *appRevisionController
}

func (l *appRevisionLister) List(namespace string, selector labels.Selector) (ret []*AppRevision, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*AppRevision))
	})
	return
}

func (l *appRevisionLister) Get(namespace, name string) (*AppRevision, error) {
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
			Group:    AppRevisionGroupVersionKind.Group,
			Resource: "appRevision",
		}, key)
	}
	return obj.(*AppRevision), nil
}

type appRevisionController struct {
	controller.GenericController
}

func (c *appRevisionController) Lister() AppRevisionLister {
	return &appRevisionLister{
		controller: c,
	}
}

func (c *appRevisionController) AddHandler(name string, handler AppRevisionHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*AppRevision))
	})
}

func (c *appRevisionController) AddClusterScopedHandler(name, cluster string, handler AppRevisionHandlerFunc) {
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

		return handler(key, obj.(*AppRevision))
	})
}

type appRevisionFactory struct {
}

func (c appRevisionFactory) Object() runtime.Object {
	return &AppRevision{}
}

func (c appRevisionFactory) List() runtime.Object {
	return &AppRevisionList{}
}

func (s *appRevisionClient) Controller() AppRevisionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.appRevisionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(AppRevisionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &appRevisionController{
		GenericController: genericController,
	}

	s.client.appRevisionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type appRevisionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AppRevisionController
}

func (s *appRevisionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *appRevisionClient) Create(o *AppRevision) (*AppRevision, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*AppRevision), err
}

func (s *appRevisionClient) Get(name string, opts metav1.GetOptions) (*AppRevision, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*AppRevision), err
}

func (s *appRevisionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*AppRevision, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*AppRevision), err
}

func (s *appRevisionClient) Update(o *AppRevision) (*AppRevision, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*AppRevision), err
}

func (s *appRevisionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *appRevisionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *appRevisionClient) List(opts metav1.ListOptions) (*AppRevisionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*AppRevisionList), err
}

func (s *appRevisionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *appRevisionClient) Patch(o *AppRevision, data []byte, subresources ...string) (*AppRevision, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*AppRevision), err
}

func (s *appRevisionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *appRevisionClient) AddHandler(name string, sync AppRevisionHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *appRevisionClient) AddLifecycle(name string, lifecycle AppRevisionLifecycle) {
	sync := NewAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *appRevisionClient) AddClusterScopedHandler(name, clusterName string, sync AppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *appRevisionClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle AppRevisionLifecycle) {
	sync := NewAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
