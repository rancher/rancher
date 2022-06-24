package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	PrincipalGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Principal",
	}
	PrincipalResource = metav1.APIResource{
		Name:         "principals",
		SingularName: "principal",
		Namespaced:   false,
		Kind:         PrincipalGroupVersionKind.Kind,
	}

	PrincipalGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "principals",
	}
)

func init() {
	resource.Put(PrincipalGroupVersionResource)
}

// Deprecated: use v3.Principal instead
type Principal = v3.Principal

func NewPrincipal(namespace, name string, obj v3.Principal) *v3.Principal {
	obj.APIVersion, obj.Kind = PrincipalGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PrincipalHandlerFunc func(key string, obj *v3.Principal) (runtime.Object, error)

type PrincipalChangeHandlerFunc func(obj *v3.Principal) (runtime.Object, error)

type PrincipalLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Principal, err error)
	Get(namespace, name string) (*v3.Principal, error)
}

type PrincipalController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PrincipalLister
	AddHandler(ctx context.Context, name string, handler PrincipalHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PrincipalHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PrincipalHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PrincipalHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PrincipalInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Principal) (*v3.Principal, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Principal, error)
	Get(name string, opts metav1.GetOptions) (*v3.Principal, error)
	Update(*v3.Principal) (*v3.Principal, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PrincipalList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PrincipalList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PrincipalController
	AddHandler(ctx context.Context, name string, sync PrincipalHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PrincipalHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PrincipalLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PrincipalLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PrincipalHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PrincipalHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PrincipalLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PrincipalLifecycle)
}

type principalLister struct {
	ns         string
	controller *principalController
}

func (l *principalLister) List(namespace string, selector labels.Selector) (ret []*v3.Principal, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Principal))
	})
	return
}

func (l *principalLister) Get(namespace, name string) (*v3.Principal, error) {
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
			Group:    PrincipalGroupVersionKind.Group,
			Resource: PrincipalGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Principal), nil
}

type principalController struct {
	ns string
	controller.GenericController
}

func (c *principalController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *principalController) Lister() PrincipalLister {
	return &principalLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *principalController) AddHandler(ctx context.Context, name string, handler PrincipalHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Principal); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *principalController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PrincipalHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Principal); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *principalController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PrincipalHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Principal); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *principalController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PrincipalHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Principal); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type principalFactory struct {
}

func (c principalFactory) Object() runtime.Object {
	return &v3.Principal{}
}

func (c principalFactory) List() runtime.Object {
	return &v3.PrincipalList{}
}

func (s *principalClient) Controller() PrincipalController {
	genericController := controller.NewGenericController(s.ns, PrincipalGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PrincipalGroupVersionResource, PrincipalGroupVersionKind.Kind, false))

	return &principalController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type principalClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PrincipalController
}

func (s *principalClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *principalClient) Create(o *v3.Principal) (*v3.Principal, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Principal), err
}

func (s *principalClient) Get(name string, opts metav1.GetOptions) (*v3.Principal, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Principal), err
}

func (s *principalClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Principal, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Principal), err
}

func (s *principalClient) Update(o *v3.Principal) (*v3.Principal, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Principal), err
}

func (s *principalClient) UpdateStatus(o *v3.Principal) (*v3.Principal, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Principal), err
}

func (s *principalClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *principalClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *principalClient) List(opts metav1.ListOptions) (*v3.PrincipalList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PrincipalList), err
}

func (s *principalClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PrincipalList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PrincipalList), err
}

func (s *principalClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *principalClient) Patch(o *v3.Principal, patchType types.PatchType, data []byte, subresources ...string) (*v3.Principal, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Principal), err
}

func (s *principalClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *principalClient) AddHandler(ctx context.Context, name string, sync PrincipalHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *principalClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PrincipalHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *principalClient) AddLifecycle(ctx context.Context, name string, lifecycle PrincipalLifecycle) {
	sync := NewPrincipalLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *principalClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PrincipalLifecycle) {
	sync := NewPrincipalLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *principalClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PrincipalHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *principalClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PrincipalHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *principalClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PrincipalLifecycle) {
	sync := NewPrincipalLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *principalClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PrincipalLifecycle) {
	sync := NewPrincipalLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
