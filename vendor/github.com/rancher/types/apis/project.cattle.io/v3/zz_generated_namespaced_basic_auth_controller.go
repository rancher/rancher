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
)

type NamespacedBasicAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedBasicAuth
}

type NamespacedBasicAuthHandlerFunc func(key string, obj *NamespacedBasicAuth) error

type NamespacedBasicAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedBasicAuth, err error)
	Get(namespace, name string) (*NamespacedBasicAuth, error)
}

type NamespacedBasicAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedBasicAuthLister
	AddHandler(name string, handler NamespacedBasicAuthHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NamespacedBasicAuthHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedBasicAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error)
	Update(*NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedBasicAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedBasicAuthController
	AddHandler(name string, sync NamespacedBasicAuthHandlerFunc)
	AddLifecycle(name string, lifecycle NamespacedBasicAuthLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NamespacedBasicAuthHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespacedBasicAuthLifecycle)
}

type namespacedBasicAuthLister struct {
	controller *namespacedBasicAuthController
}

func (l *namespacedBasicAuthLister) List(namespace string, selector labels.Selector) (ret []*NamespacedBasicAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedBasicAuth))
	})
	return
}

func (l *namespacedBasicAuthLister) Get(namespace, name string) (*NamespacedBasicAuth, error) {
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
			Resource: "namespacedBasicAuth",
		}, key)
	}
	return obj.(*NamespacedBasicAuth), nil
}

type namespacedBasicAuthController struct {
	controller.GenericController
}

func (c *namespacedBasicAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedBasicAuthController) Lister() NamespacedBasicAuthLister {
	return &namespacedBasicAuthLister{
		controller: c,
	}
}

func (c *namespacedBasicAuthController) AddHandler(name string, handler NamespacedBasicAuthHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NamespacedBasicAuth))
	})
}

func (c *namespacedBasicAuthController) AddClusterScopedHandler(name, cluster string, handler NamespacedBasicAuthHandlerFunc) {
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

		return handler(key, obj.(*NamespacedBasicAuth))
	})
}

type namespacedBasicAuthFactory struct {
}

func (c namespacedBasicAuthFactory) Object() runtime.Object {
	return &NamespacedBasicAuth{}
}

func (c namespacedBasicAuthFactory) List() runtime.Object {
	return &NamespacedBasicAuthList{}
}

func (s *namespacedBasicAuthClient) Controller() NamespacedBasicAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedBasicAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedBasicAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedBasicAuthController{
		GenericController: genericController,
	}

	s.client.namespacedBasicAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *namespacedBasicAuthClient) Create(o *NamespacedBasicAuth) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Get(name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Update(o *NamespacedBasicAuth) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedBasicAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedBasicAuthClient) List(opts metav1.ListOptions) (*NamespacedBasicAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedBasicAuthList), err
}

func (s *namespacedBasicAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedBasicAuthClient) Patch(o *NamespacedBasicAuth, data []byte, subresources ...string) (*NamespacedBasicAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NamespacedBasicAuth), err
}

func (s *namespacedBasicAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedBasicAuthClient) AddHandler(name string, sync NamespacedBasicAuthHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *namespacedBasicAuthClient) AddLifecycle(name string, lifecycle NamespacedBasicAuthLifecycle) {
	sync := NewNamespacedBasicAuthLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *namespacedBasicAuthClient) AddClusterScopedHandler(name, clusterName string, sync NamespacedBasicAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *namespacedBasicAuthClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespacedBasicAuthLifecycle) {
	sync := NewNamespacedBasicAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
