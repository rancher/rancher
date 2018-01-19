package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type BasicAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BasicAuth
}

type BasicAuthHandlerFunc func(key string, obj *BasicAuth) error

type BasicAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*BasicAuth, err error)
	Get(namespace, name string) (*BasicAuth, error)
}

type BasicAuthController interface {
	Informer() cache.SharedIndexInformer
	Lister() BasicAuthLister
	AddHandler(name string, handler BasicAuthHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler BasicAuthHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type BasicAuthInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*BasicAuth) (*BasicAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*BasicAuth, error)
	Get(name string, opts metav1.GetOptions) (*BasicAuth, error)
	Update(*BasicAuth) (*BasicAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*BasicAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() BasicAuthController
	AddHandler(name string, sync BasicAuthHandlerFunc)
	AddLifecycle(name string, lifecycle BasicAuthLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync BasicAuthHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle BasicAuthLifecycle)
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
		}, name)
	}
	return obj.(*BasicAuth), nil
}

type basicAuthController struct {
	controller.GenericController
}

func (c *basicAuthController) Lister() BasicAuthLister {
	return &basicAuthLister{
		controller: c,
	}
}

func (c *basicAuthController) AddHandler(name string, handler BasicAuthHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*BasicAuth))
	})
}

func (c *basicAuthController) AddClusterScopedHandler(name, cluster string, handler BasicAuthHandlerFunc) {
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

		return handler(key, obj.(*BasicAuth))
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
	objectClient *clientbase.ObjectClient
	controller   BasicAuthController
}

func (s *basicAuthClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *basicAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *basicAuthClient) Patch(o *BasicAuth, data []byte, subresources ...string) (*BasicAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*BasicAuth), err
}

func (s *basicAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *basicAuthClient) AddHandler(name string, sync BasicAuthHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *basicAuthClient) AddLifecycle(name string, lifecycle BasicAuthLifecycle) {
	sync := NewBasicAuthLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *basicAuthClient) AddClusterScopedHandler(name, clusterName string, sync BasicAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *basicAuthClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle BasicAuthLifecycle) {
	sync := NewBasicAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
