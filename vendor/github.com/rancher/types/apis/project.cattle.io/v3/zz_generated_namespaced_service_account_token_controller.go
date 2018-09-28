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
	NamespacedServiceAccountTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedServiceAccountToken",
	}
	NamespacedServiceAccountTokenResource = metav1.APIResource{
		Name:         "namespacedserviceaccounttokens",
		SingularName: "namespacedserviceaccounttoken",
		Namespaced:   true,

		Kind: NamespacedServiceAccountTokenGroupVersionKind.Kind,
	}
)

type NamespacedServiceAccountTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedServiceAccountToken
}

type NamespacedServiceAccountTokenHandlerFunc func(key string, obj *NamespacedServiceAccountToken) error

type NamespacedServiceAccountTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedServiceAccountToken, err error)
	Get(namespace, name string) (*NamespacedServiceAccountToken, error)
}

type NamespacedServiceAccountTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedServiceAccountTokenLister
	AddHandler(name string, handler NamespacedServiceAccountTokenHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NamespacedServiceAccountTokenHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedServiceAccountTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error)
	Update(*NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedServiceAccountTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedServiceAccountTokenController
	AddHandler(name string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddLifecycle(name string, lifecycle NamespacedServiceAccountTokenLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespacedServiceAccountTokenLifecycle)
}

type namespacedServiceAccountTokenLister struct {
	controller *namespacedServiceAccountTokenController
}

func (l *namespacedServiceAccountTokenLister) List(namespace string, selector labels.Selector) (ret []*NamespacedServiceAccountToken, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedServiceAccountToken))
	})
	return
}

func (l *namespacedServiceAccountTokenLister) Get(namespace, name string) (*NamespacedServiceAccountToken, error) {
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
			Group:    NamespacedServiceAccountTokenGroupVersionKind.Group,
			Resource: "namespacedServiceAccountToken",
		}, key)
	}
	return obj.(*NamespacedServiceAccountToken), nil
}

type namespacedServiceAccountTokenController struct {
	controller.GenericController
}

func (c *namespacedServiceAccountTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedServiceAccountTokenController) Lister() NamespacedServiceAccountTokenLister {
	return &namespacedServiceAccountTokenLister{
		controller: c,
	}
}

func (c *namespacedServiceAccountTokenController) AddHandler(name string, handler NamespacedServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NamespacedServiceAccountToken))
	})
}

func (c *namespacedServiceAccountTokenController) AddClusterScopedHandler(name, cluster string, handler NamespacedServiceAccountTokenHandlerFunc) {
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

		return handler(key, obj.(*NamespacedServiceAccountToken))
	})
}

type namespacedServiceAccountTokenFactory struct {
}

func (c namespacedServiceAccountTokenFactory) Object() runtime.Object {
	return &NamespacedServiceAccountToken{}
}

func (c namespacedServiceAccountTokenFactory) List() runtime.Object {
	return &NamespacedServiceAccountTokenList{}
}

func (s *namespacedServiceAccountTokenClient) Controller() NamespacedServiceAccountTokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedServiceAccountTokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedServiceAccountTokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedServiceAccountTokenController{
		GenericController: genericController,
	}

	s.client.namespacedServiceAccountTokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedServiceAccountTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedServiceAccountTokenController
}

func (s *namespacedServiceAccountTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedServiceAccountTokenClient) Create(o *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) Get(name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) Update(o *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedServiceAccountTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedServiceAccountTokenClient) List(opts metav1.ListOptions) (*NamespacedServiceAccountTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedServiceAccountTokenList), err
}

func (s *namespacedServiceAccountTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedServiceAccountTokenClient) Patch(o *NamespacedServiceAccountToken, data []byte, subresources ...string) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedServiceAccountTokenClient) AddHandler(name string, sync NamespacedServiceAccountTokenHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *namespacedServiceAccountTokenClient) AddLifecycle(name string, lifecycle NamespacedServiceAccountTokenLifecycle) {
	sync := NewNamespacedServiceAccountTokenLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *namespacedServiceAccountTokenClient) AddClusterScopedHandler(name, clusterName string, sync NamespacedServiceAccountTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *namespacedServiceAccountTokenClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NamespacedServiceAccountTokenLifecycle) {
	sync := NewNamespacedServiceAccountTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
