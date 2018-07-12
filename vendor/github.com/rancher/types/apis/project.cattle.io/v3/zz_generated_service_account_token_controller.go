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
	ServiceAccountTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ServiceAccountToken",
	}
	ServiceAccountTokenResource = metav1.APIResource{
		Name:         "serviceaccounttokens",
		SingularName: "serviceaccounttoken",
		Namespaced:   true,

		Kind: ServiceAccountTokenGroupVersionKind.Kind,
	}
)

type ServiceAccountTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceAccountToken
}

type ServiceAccountTokenHandlerFunc func(key string, obj *ServiceAccountToken) error

type ServiceAccountTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*ServiceAccountToken, err error)
	Get(namespace, name string) (*ServiceAccountToken, error)
}

type ServiceAccountTokenController interface {
	Informer() cache.SharedIndexInformer
	Lister() ServiceAccountTokenLister
	AddHandler(name string, handler ServiceAccountTokenHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ServiceAccountTokenHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ServiceAccountTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ServiceAccountToken) (*ServiceAccountToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ServiceAccountToken, error)
	Get(name string, opts metav1.GetOptions) (*ServiceAccountToken, error)
	Update(*ServiceAccountToken) (*ServiceAccountToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ServiceAccountTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceAccountTokenController
	AddHandler(name string, sync ServiceAccountTokenHandlerFunc)
	AddLifecycle(name string, lifecycle ServiceAccountTokenLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ServiceAccountTokenHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ServiceAccountTokenLifecycle)
}

type serviceAccountTokenLister struct {
	controller *serviceAccountTokenController
}

func (l *serviceAccountTokenLister) List(namespace string, selector labels.Selector) (ret []*ServiceAccountToken, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ServiceAccountToken))
	})
	return
}

func (l *serviceAccountTokenLister) Get(namespace, name string) (*ServiceAccountToken, error) {
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
			Group:    ServiceAccountTokenGroupVersionKind.Group,
			Resource: "serviceAccountToken",
		}, key)
	}
	return obj.(*ServiceAccountToken), nil
}

type serviceAccountTokenController struct {
	controller.GenericController
}

func (c *serviceAccountTokenController) Lister() ServiceAccountTokenLister {
	return &serviceAccountTokenLister{
		controller: c,
	}
}

func (c *serviceAccountTokenController) AddHandler(name string, handler ServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ServiceAccountToken))
	})
}

func (c *serviceAccountTokenController) AddClusterScopedHandler(name, cluster string, handler ServiceAccountTokenHandlerFunc) {
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

		return handler(key, obj.(*ServiceAccountToken))
	})
}

type serviceAccountTokenFactory struct {
}

func (c serviceAccountTokenFactory) Object() runtime.Object {
	return &ServiceAccountToken{}
}

func (c serviceAccountTokenFactory) List() runtime.Object {
	return &ServiceAccountTokenList{}
}

func (s *serviceAccountTokenClient) Controller() ServiceAccountTokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.serviceAccountTokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ServiceAccountTokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &serviceAccountTokenController{
		GenericController: genericController,
	}

	s.client.serviceAccountTokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type serviceAccountTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceAccountTokenController
}

func (s *serviceAccountTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *serviceAccountTokenClient) Create(o *ServiceAccountToken) (*ServiceAccountToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) Get(name string, opts metav1.GetOptions) (*ServiceAccountToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ServiceAccountToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) Update(o *ServiceAccountToken) (*ServiceAccountToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceAccountTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceAccountTokenClient) List(opts metav1.ListOptions) (*ServiceAccountTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ServiceAccountTokenList), err
}

func (s *serviceAccountTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceAccountTokenClient) Patch(o *ServiceAccountToken, data []byte, subresources ...string) (*ServiceAccountToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceAccountTokenClient) AddHandler(name string, sync ServiceAccountTokenHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *serviceAccountTokenClient) AddLifecycle(name string, lifecycle ServiceAccountTokenLifecycle) {
	sync := NewServiceAccountTokenLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *serviceAccountTokenClient) AddClusterScopedHandler(name, clusterName string, sync ServiceAccountTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *serviceAccountTokenClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ServiceAccountTokenLifecycle) {
	sync := NewServiceAccountTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
