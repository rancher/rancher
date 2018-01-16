package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type NamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Namespace
}

type NamespaceHandlerFunc func(key string, obj *v1.Namespace) error

type NamespaceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Namespace, err error)
	Get(namespace, name string) (*v1.Namespace, error)
}

type NamespaceController interface {
	Informer() cache.SharedIndexInformer
	Lister() NamespaceLister
	AddHandler(name string, handler NamespaceHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NamespaceHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespaceInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.Namespace) (*v1.Namespace, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Namespace, error)
	Get(name string, opts metav1.GetOptions) (*v1.Namespace, error)
	Update(*v1.Namespace) (*v1.Namespace, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespaceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespaceController
	AddHandler(name string, sync NamespaceHandlerFunc)
	AddLifecycle(name string, lifecycle NamespaceLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NamespaceHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespaceLifecycle)
}

type namespaceLister struct {
	controller *namespaceController
}

func (l *namespaceLister) List(namespace string, selector labels.Selector) (ret []*v1.Namespace, err error) {
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
			Resource: "namespace",
		}, name)
	}
	return obj.(*v1.Namespace), nil
}

type namespaceController struct {
	controller.GenericController
}

func (c *namespaceController) Lister() NamespaceLister {
	return &namespaceLister{
		controller: c,
	}
}

func (c *namespaceController) AddHandler(name string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Namespace))
	})
}

func (c *namespaceController) AddClusterScopedHandler(name, cluster string, handler NamespaceHandlerFunc) {
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

		return handler(key, obj.(*v1.Namespace))
	})
}

type namespaceFactory struct {
}

func (c namespaceFactory) Object() runtime.Object {
	return &v1.Namespace{}
}

func (c namespaceFactory) List() runtime.Object {
	return &NamespaceList{}
}

func (s *namespaceClient) Controller() NamespaceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespaceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespaceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespaceController{
		GenericController: genericController,
	}

	s.client.namespaceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespaceClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   NamespaceController
}

func (s *namespaceClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *namespaceClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Namespace, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Update(o *v1.Namespace) (*v1.Namespace, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespaceClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *namespaceClient) List(opts metav1.ListOptions) (*NamespaceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespaceList), err
}

func (s *namespaceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespaceClient) Patch(o *v1.Namespace, data []byte, subresources ...string) (*v1.Namespace, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespaceClient) AddHandler(name string, sync NamespaceHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *namespaceClient) AddLifecycle(name string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *namespaceClient) AddClusterScopedHandler(name, clusterName string, sync NamespaceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
