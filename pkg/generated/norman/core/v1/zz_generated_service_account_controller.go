package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
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

	ServiceAccountGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "serviceaccounts",
	}
)

func init() {
	resource.Put(ServiceAccountGroupVersionResource)
}

// Deprecated: use v1.ServiceAccount instead
type ServiceAccount = v1.ServiceAccount

func NewServiceAccount(namespace, name string, obj v1.ServiceAccount) *v1.ServiceAccount {
	obj.APIVersion, obj.Kind = ServiceAccountGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceAccountHandlerFunc func(key string, obj *v1.ServiceAccount) (runtime.Object, error)

type ServiceAccountChangeHandlerFunc func(obj *v1.ServiceAccount) (runtime.Object, error)

type ServiceAccountLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ServiceAccount, err error)
	Get(namespace, name string) (*v1.ServiceAccount, error)
}

type ServiceAccountController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceAccountLister
	AddHandler(ctx context.Context, name string, handler ServiceAccountHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceAccountHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceAccountHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ServiceAccountHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ServiceAccountInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Get(name string, opts metav1.GetOptions) (*v1.ServiceAccount, error)
	Update(*v1.ServiceAccount) (*v1.ServiceAccount, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ServiceAccountList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ServiceAccountList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceAccountController
	AddHandler(ctx context.Context, name string, sync ServiceAccountHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceAccountHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceAccountLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceAccountLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceAccountHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceAccountHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceAccountLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceAccountLifecycle)
}

type serviceAccountLister struct {
	ns         string
	controller *serviceAccountController
}

func (l *serviceAccountLister) List(namespace string, selector labels.Selector) (ret []*v1.ServiceAccount, err error) {
	if namespace == "" {
		namespace = l.ns
	}
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
			Resource: ServiceAccountGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ServiceAccount), nil
}

type serviceAccountController struct {
	ns string
	controller.GenericController
}

func (c *serviceAccountController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceAccountController) Lister() ServiceAccountLister {
	return &serviceAccountLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *serviceAccountController) AddHandler(ctx context.Context, name string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceAccount); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceAccount); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceAccount); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ServiceAccountHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceAccount); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceAccountFactory struct {
}

func (c serviceAccountFactory) Object() runtime.Object {
	return &v1.ServiceAccount{}
}

func (c serviceAccountFactory) List() runtime.Object {
	return &v1.ServiceAccountList{}
}

func (s *serviceAccountClient) Controller() ServiceAccountController {
	genericController := controller.NewGenericController(s.ns, ServiceAccountGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ServiceAccountGroupVersionResource, ServiceAccountGroupVersionKind.Kind, true))

	return &serviceAccountController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *serviceAccountClient) UpdateStatus(o *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceAccountClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceAccountClient) List(opts metav1.ListOptions) (*v1.ServiceAccountList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ServiceAccountList), err
}

func (s *serviceAccountClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ServiceAccountList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ServiceAccountList), err
}

func (s *serviceAccountClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceAccountClient) Patch(o *v1.ServiceAccount, patchType types.PatchType, data []byte, subresources ...string) (*v1.ServiceAccount, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ServiceAccount), err
}

func (s *serviceAccountClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceAccountClient) AddHandler(ctx context.Context, name string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceAccountClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceAccountClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceAccountClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceAccountClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceAccountClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceAccountHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *serviceAccountClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceAccountClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceAccountLifecycle) {
	sync := NewServiceAccountLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
