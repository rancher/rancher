package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
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
	ServiceAccountGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ServiceAccount",
	}
	ServiceAccountResource = metav1.APIResource{
		Name:         "serviceaccounts",
		SingularName: "serviceaccount",
		Namespaced:   true,

		Kind: ServiceAccountGroupVersionKind.Kind,
	}
)

type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ServiceAccount
}

type ServiceAccountHandlerFunc func(key string, obj *v1.ServiceAccount) error

type ServiceAccountLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ServiceAccount, err error)
	Get(namespace, name string) (*v1.ServiceAccount, error)
}

type ServiceAccountController interface {
	Informer() cache.SharedIndexInformer
	Lister() ServiceAccountLister
	AddHandler(name string, handler ServiceAccountHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ServiceAccountHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ServiceAccountInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Get(name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Update(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ServiceAccountList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceAccountController
	AddHandler(name string, sync ServiceAccountHandlerFunc)
	AddLifecycle(name string, lifecycle ServiceAccountLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ServiceAccountHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ServiceAccountLifecycle)
}

type serviceAccountLister struct {
	controller *serviceAccountController
}

func (l *serviceAccountLister) List(namespace string, selector labels.Selector) (ret []*v1.ServiceAccount, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ServiceAccount))
	})
	return
}

func (l *serviceAccountLister) Get(namespace, name string) (*v1.ServiceAccount, error) {
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
			Group:    ServiceAccountGroupVersionKind.Group,
			Resource: "serviceAccount",
		}, key)
	}
	return obj.(*v1.ServiceAccount), nil
}

type serviceAccountController struct {
	controller.GenericController
}

func (c *serviceAccountController) Lister() ServiceAccountLister {
	return &serviceAccountLister{
		controller: c,
	}
}

func (c *serviceAccountController) AddHandler(name string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.ServiceAccount))
	})
}

func (c *serviceAccountController) AddClusterScopedHandler(name, cluster string, handler ServiceAccountHandlerFunc) {
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

		return handler(key, obj.(*v1.ServiceAccount))
	})
}

type serviceAccountFactory struct {
}

func (c serviceAccountFactory) Object() runtime.Object {
	return &v1.ServiceAccount{}
}

func (c serviceAccountFactory) List() runtime.Object {
	return &ServiceAccountList{}
}

func (s *serviceAccountClient) Controller() ServiceAccountController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.serviceAccountControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ServiceAccountGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &serviceAccountController{
		GenericController: genericController,
	}

	s.client.serviceAccountControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type serviceAccountClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceAccountController
}

func (s *serviceAccountClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *serviceAccountClient) Create(o *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Get(name string, opts metav1.GetOptions) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Update(o *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceAccountClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceAccountClient) List(opts metav1.ListOptions) (*ServiceAccountList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ServiceAccountList), err
}

func (s *serviceAccountClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceAccountClient) Patch(o *v1.ServiceAccount, data []byte, subresources ...string) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceAccountClient) AddHandler(name string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *serviceAccountClient) AddLifecycle(name string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *serviceAccountClient) AddClusterScopedHandler(name, clusterName string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *serviceAccountClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
