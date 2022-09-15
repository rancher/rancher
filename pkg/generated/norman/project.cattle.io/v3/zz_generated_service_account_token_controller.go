package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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

	ServiceAccountTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "serviceaccounttokens",
	}
)

func init() {
	resource.Put(ServiceAccountTokenGroupVersionResource)
}

// Deprecated: use v3.ServiceAccountToken instead
type ServiceAccountToken = v3.ServiceAccountToken

func NewServiceAccountToken(namespace, name string, obj v3.ServiceAccountToken) *v3.ServiceAccountToken {
	obj.APIVersion, obj.Kind = ServiceAccountTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceAccountTokenHandlerFunc func(key string, obj *v3.ServiceAccountToken) (runtime.Object, error)

type ServiceAccountTokenChangeHandlerFunc func(obj *v3.ServiceAccountToken) (runtime.Object, error)

type ServiceAccountTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ServiceAccountToken, err error)
	Get(namespace, name string) (*v3.ServiceAccountToken, error)
}

type ServiceAccountTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceAccountTokenLister
	AddHandler(ctx context.Context, name string, handler ServiceAccountTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceAccountTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceAccountTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ServiceAccountTokenHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ServiceAccountTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ServiceAccountToken) (*v3.ServiceAccountToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ServiceAccountToken, error)
	Get(name string, opts metav1.GetOptions) (*v3.ServiceAccountToken, error)
	Update(*v3.ServiceAccountToken) (*v3.ServiceAccountToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ServiceAccountTokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ServiceAccountTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceAccountTokenController
	AddHandler(ctx context.Context, name string, sync ServiceAccountTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceAccountTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceAccountTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceAccountTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceAccountTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceAccountTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceAccountTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceAccountTokenLifecycle)
}

type serviceAccountTokenLister struct {
	ns         string
	controller *serviceAccountTokenController
}

func (l *serviceAccountTokenLister) List(namespace string, selector labels.Selector) (ret []*v3.ServiceAccountToken, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ServiceAccountToken))
	})
	return
}

func (l *serviceAccountTokenLister) Get(namespace, name string) (*v3.ServiceAccountToken, error) {
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
			Resource: ServiceAccountTokenGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ServiceAccountToken), nil
}

type serviceAccountTokenController struct {
	ns string
	controller.GenericController
}

func (c *serviceAccountTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceAccountTokenController) Lister() ServiceAccountTokenLister {
	return &serviceAccountTokenLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *serviceAccountTokenController) AddHandler(ctx context.Context, name string, handler ServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ServiceAccountToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ServiceAccountToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ServiceAccountToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceAccountTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ServiceAccountToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceAccountTokenFactory struct {
}

func (c serviceAccountTokenFactory) Object() runtime.Object {
	return &v3.ServiceAccountToken{}
}

func (c serviceAccountTokenFactory) List() runtime.Object {
	return &v3.ServiceAccountTokenList{}
}

func (s *serviceAccountTokenClient) Controller() ServiceAccountTokenController {
	genericController := controller.NewGenericController(s.ns, ServiceAccountTokenGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ServiceAccountTokenGroupVersionResource, ServiceAccountTokenGroupVersionKind.Kind, true))

	return &serviceAccountTokenController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *serviceAccountTokenClient) Create(o *v3.ServiceAccountToken) (*v3.ServiceAccountToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) Get(name string, opts metav1.GetOptions) (*v3.ServiceAccountToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ServiceAccountToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) Update(o *v3.ServiceAccountToken) (*v3.ServiceAccountToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) UpdateStatus(o *v3.ServiceAccountToken) (*v3.ServiceAccountToken, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceAccountTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceAccountTokenClient) List(opts metav1.ListOptions) (*v3.ServiceAccountTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ServiceAccountTokenList), err
}

func (s *serviceAccountTokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ServiceAccountTokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ServiceAccountTokenList), err
}

func (s *serviceAccountTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceAccountTokenClient) Patch(o *v3.ServiceAccountToken, patchType types.PatchType, data []byte, subresources ...string) (*v3.ServiceAccountToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ServiceAccountToken), err
}

func (s *serviceAccountTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceAccountTokenClient) AddHandler(ctx context.Context, name string, sync ServiceAccountTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceAccountTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceAccountTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceAccountTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceAccountTokenLifecycle) {
	sync := NewServiceAccountTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceAccountTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceAccountTokenLifecycle) {
	sync := NewServiceAccountTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceAccountTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceAccountTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceAccountTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceAccountTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *serviceAccountTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceAccountTokenLifecycle) {
	sync := NewServiceAccountTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceAccountTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceAccountTokenLifecycle) {
	sync := NewServiceAccountTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
