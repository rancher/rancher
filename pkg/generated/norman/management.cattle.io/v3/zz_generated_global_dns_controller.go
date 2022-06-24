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
	GlobalDnsGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalDns",
	}
	GlobalDnsResource = metav1.APIResource{
		Name:         "globaldnses",
		SingularName: "globaldns",
		Namespaced:   true,

		Kind: GlobalDnsGroupVersionKind.Kind,
	}

	GlobalDnsGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globaldnses",
	}
)

func init() {
	resource.Put(GlobalDnsGroupVersionResource)
}

// Deprecated: use v3.GlobalDns instead
type GlobalDns = v3.GlobalDns

func NewGlobalDns(namespace, name string, obj v3.GlobalDns) *v3.GlobalDns {
	obj.APIVersion, obj.Kind = GlobalDnsGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalDnsHandlerFunc func(key string, obj *v3.GlobalDns) (runtime.Object, error)

type GlobalDnsChangeHandlerFunc func(obj *v3.GlobalDns) (runtime.Object, error)

type GlobalDnsLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.GlobalDns, err error)
	Get(namespace, name string) (*v3.GlobalDns, error)
}

type GlobalDnsController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalDnsLister
	AddHandler(ctx context.Context, name string, handler GlobalDnsHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDnsHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalDnsHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalDnsHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type GlobalDnsInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.GlobalDns) (*v3.GlobalDns, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalDns, error)
	Get(name string, opts metav1.GetOptions) (*v3.GlobalDns, error)
	Update(*v3.GlobalDns) (*v3.GlobalDns, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.GlobalDnsList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalDnsList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalDnsController
	AddHandler(ctx context.Context, name string, sync GlobalDnsHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDnsHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalDnsLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDnsLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDnsHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDnsHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDnsLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDnsLifecycle)
}

type globalDnsLister struct {
	ns         string
	controller *globalDnsController
}

func (l *globalDnsLister) List(namespace string, selector labels.Selector) (ret []*v3.GlobalDns, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.GlobalDns))
	})
	return
}

func (l *globalDnsLister) Get(namespace, name string) (*v3.GlobalDns, error) {
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
			Group:    GlobalDnsGroupVersionKind.Group,
			Resource: GlobalDnsGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.GlobalDns), nil
}

type globalDnsController struct {
	ns string
	controller.GenericController
}

func (c *globalDnsController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalDnsController) Lister() GlobalDnsLister {
	return &globalDnsLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *globalDnsController) AddHandler(ctx context.Context, name string, handler GlobalDnsHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDns); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalDnsHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDns); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalDnsHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDns); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalDnsHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalDns); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalDnsFactory struct {
}

func (c globalDnsFactory) Object() runtime.Object {
	return &v3.GlobalDns{}
}

func (c globalDnsFactory) List() runtime.Object {
	return &v3.GlobalDnsList{}
}

func (s *globalDnsClient) Controller() GlobalDnsController {
	genericController := controller.NewGenericController(s.ns, GlobalDnsGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(GlobalDnsGroupVersionResource, GlobalDnsGroupVersionKind.Kind, true))

	return &globalDnsController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type globalDnsClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalDnsController
}

func (s *globalDnsClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalDnsClient) Create(o *v3.GlobalDns) (*v3.GlobalDns, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.GlobalDns), err
}

func (s *globalDnsClient) Get(name string, opts metav1.GetOptions) (*v3.GlobalDns, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.GlobalDns), err
}

func (s *globalDnsClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalDns, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.GlobalDns), err
}

func (s *globalDnsClient) Update(o *v3.GlobalDns) (*v3.GlobalDns, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.GlobalDns), err
}

func (s *globalDnsClient) UpdateStatus(o *v3.GlobalDns) (*v3.GlobalDns, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.GlobalDns), err
}

func (s *globalDnsClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalDnsClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalDnsClient) List(opts metav1.ListOptions) (*v3.GlobalDnsList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.GlobalDnsList), err
}

func (s *globalDnsClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalDnsList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.GlobalDnsList), err
}

func (s *globalDnsClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalDnsClient) Patch(o *v3.GlobalDns, patchType types.PatchType, data []byte, subresources ...string) (*v3.GlobalDns, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.GlobalDns), err
}

func (s *globalDnsClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalDnsClient) AddHandler(ctx context.Context, name string, sync GlobalDnsHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDnsHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalDnsLifecycle) {
	sync := NewGlobalDnsLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDnsLifecycle) {
	sync := NewGlobalDnsLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDnsHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDnsHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDnsLifecycle) {
	sync := NewGlobalDnsLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDnsLifecycle) {
	sync := NewGlobalDnsLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
