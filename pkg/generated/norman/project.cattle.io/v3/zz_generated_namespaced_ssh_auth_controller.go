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

	NamespacedSSHAuthGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespacedsshauths",
	}
)

func init() {
	resource.Put(NamespacedSSHAuthGroupVersionResource)
}

// Deprecated: use v3.NamespacedSSHAuth instead
type NamespacedSSHAuth = v3.NamespacedSSHAuth

func NewNamespacedSSHAuth(namespace, name string, obj v3.NamespacedSSHAuth) *v3.NamespacedSSHAuth {
	obj.APIVersion, obj.Kind = NamespacedSSHAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedSSHAuthHandlerFunc func(key string, obj *v3.NamespacedSSHAuth) (runtime.Object, error)

type NamespacedSSHAuthChangeHandlerFunc func(obj *v3.NamespacedSSHAuth) (runtime.Object, error)

type NamespacedSSHAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NamespacedSSHAuth, err error)
	Get(namespace, name string) (*v3.NamespacedSSHAuth, error)
}

type NamespacedSSHAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedSSHAuthLister
	AddHandler(ctx context.Context, name string, handler NamespacedSSHAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedSSHAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedSSHAuthHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NamespacedSSHAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NamespacedSSHAuth) (*v3.NamespacedSSHAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedSSHAuth, error)
	Get(name string, opts metav1.GetOptions) (*v3.NamespacedSSHAuth, error)
	Update(*v3.NamespacedSSHAuth) (*v3.NamespacedSSHAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NamespacedSSHAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedSSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedSSHAuthController
	AddHandler(ctx context.Context, name string, sync NamespacedSSHAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedSSHAuthHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedSSHAuthLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedSSHAuthLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle)
}

type namespacedSshAuthLister struct {
	ns         string
	controller *namespacedSshAuthController
}

func (l *namespacedSshAuthLister) List(namespace string, selector labels.Selector) (ret []*v3.NamespacedSSHAuth, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NamespacedSSHAuth))
	})
	return
}

func (l *namespacedSshAuthLister) Get(namespace, name string) (*v3.NamespacedSSHAuth, error) {
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
			Resource: NamespacedSSHAuthGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NamespacedSSHAuth), nil
}

type namespacedSshAuthController struct {
	ns string
	controller.GenericController
}

func (c *namespacedSshAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedSshAuthController) Lister() NamespacedSSHAuthLister {
	return &namespacedSshAuthLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *namespacedSshAuthController) AddHandler(ctx context.Context, name string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedSSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedSshAuthController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedSSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedSshAuthController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedSSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedSshAuthController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedSSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedSshAuthFactory struct {
}

func (c namespacedSshAuthFactory) Object() runtime.Object {
	return &v3.NamespacedSSHAuth{}
}

func (c namespacedSshAuthFactory) List() runtime.Object {
	return &v3.NamespacedSSHAuthList{}
}

func (s *namespacedSshAuthClient) Controller() NamespacedSSHAuthController {
	genericController := controller.NewGenericController(s.ns, NamespacedSSHAuthGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NamespacedSSHAuthGroupVersionResource, NamespacedSSHAuthGroupVersionKind.Kind, true))

	return &namespacedSshAuthController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *namespacedSshAuthClient) Create(o *v3.NamespacedSSHAuth) (*v3.NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Get(name string, opts metav1.GetOptions) (*v3.NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedSSHAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Update(o *v3.NamespacedSSHAuth) (*v3.NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) UpdateStatus(o *v3.NamespacedSSHAuth) (*v3.NamespacedSSHAuth, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedSshAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedSshAuthClient) List(opts metav1.ListOptions) (*v3.NamespacedSSHAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NamespacedSSHAuthList), err
}

func (s *namespacedSshAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedSSHAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NamespacedSSHAuthList), err
}

func (s *namespacedSshAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedSshAuthClient) Patch(o *v3.NamespacedSSHAuth, patchType types.PatchType, data []byte, subresources ...string) (*v3.NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedSshAuthClient) AddHandler(ctx context.Context, name string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedSshAuthClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedSshAuthClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedSshAuthClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
