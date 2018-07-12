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
	NamespacedSSHAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedSSHAuth",
	}
	NamespacedSSHAuthResource = metav1.APIResource{
		Name:         "namespacedsshauths",
		SingularName: "namespacedsshauth",
		Namespaced:   true,

		Kind: NamespacedSSHAuthGroupVersionKind.Kind,
	}
)

type NamespacedSSHAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedSSHAuth
}

type NamespacedSSHAuthHandlerFunc func(key string, obj *NamespacedSSHAuth) error

type NamespacedSSHAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedSSHAuth, err error)
	Get(namespace, name string) (*NamespacedSSHAuth, error)
}

type NamespacedSSHAuthController interface {
	Informer() cache.SharedIndexInformer
	Lister() NamespacedSSHAuthLister
	AddHandler(name string, handler NamespacedSSHAuthHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NamespacedSSHAuthHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedSSHAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error)
	Update(*NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedSSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedSSHAuthController
	AddHandler(name string, sync NamespacedSSHAuthHandlerFunc)
	AddLifecycle(name string, lifecycle NamespacedSSHAuthLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespacedSSHAuthLifecycle)
}

type namespacedSshAuthLister struct {
	controller *namespacedSshAuthController
}

func (l *namespacedSshAuthLister) List(namespace string, selector labels.Selector) (ret []*NamespacedSSHAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedSSHAuth))
	})
	return
}

func (l *namespacedSshAuthLister) Get(namespace, name string) (*NamespacedSSHAuth, error) {
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
			Group:    NamespacedSSHAuthGroupVersionKind.Group,
			Resource: "namespacedSshAuth",
		}, key)
	}
	return obj.(*NamespacedSSHAuth), nil
}

type namespacedSshAuthController struct {
	controller.GenericController
}

func (c *namespacedSshAuthController) Lister() NamespacedSSHAuthLister {
	return &namespacedSshAuthLister{
		controller: c,
	}
}

func (c *namespacedSshAuthController) AddHandler(name string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NamespacedSSHAuth))
	})
}

func (c *namespacedSshAuthController) AddClusterScopedHandler(name, cluster string, handler NamespacedSSHAuthHandlerFunc) {
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

		return handler(key, obj.(*NamespacedSSHAuth))
	})
}

type namespacedSshAuthFactory struct {
}

func (c namespacedSshAuthFactory) Object() runtime.Object {
	return &NamespacedSSHAuth{}
}

func (c namespacedSshAuthFactory) List() runtime.Object {
	return &NamespacedSSHAuthList{}
}

func (s *namespacedSshAuthClient) Controller() NamespacedSSHAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedSshAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedSSHAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedSshAuthController{
		GenericController: genericController,
	}

	s.client.namespacedSshAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedSshAuthClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedSSHAuthController
}

func (s *namespacedSshAuthClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedSshAuthClient) Create(o *NamespacedSSHAuth) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Get(name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Update(o *NamespacedSSHAuth) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedSshAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedSshAuthClient) List(opts metav1.ListOptions) (*NamespacedSSHAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedSSHAuthList), err
}

func (s *namespacedSshAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedSshAuthClient) Patch(o *NamespacedSSHAuth, data []byte, subresources ...string) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedSshAuthClient) AddHandler(name string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *namespacedSshAuthClient) AddLifecycle(name string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedHandler(name, clusterName string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
