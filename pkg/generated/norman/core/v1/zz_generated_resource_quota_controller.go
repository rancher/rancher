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
	ResourceQuotaGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ResourceQuota",
	}
	ResourceQuotaResource = metav1.APIResource{
		Name:         "resourcequotas",
		SingularName: "resourcequota",
		Namespaced:   true,

		Kind: ResourceQuotaGroupVersionKind.Kind,
	}

	ResourceQuotaGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "resourcequotas",
	}
)

func init() {
	resource.Put(ResourceQuotaGroupVersionResource)
}

// Deprecated: use v1.ResourceQuota instead
type ResourceQuota = v1.ResourceQuota

func NewResourceQuota(namespace, name string, obj v1.ResourceQuota) *v1.ResourceQuota {
	obj.APIVersion, obj.Kind = ResourceQuotaGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ResourceQuotaHandlerFunc func(key string, obj *v1.ResourceQuota) (runtime.Object, error)

type ResourceQuotaChangeHandlerFunc func(obj *v1.ResourceQuota) (runtime.Object, error)

type ResourceQuotaLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ResourceQuota, err error)
	Get(namespace, name string) (*v1.ResourceQuota, error)
}

type ResourceQuotaController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ResourceQuotaLister
	AddHandler(ctx context.Context, name string, handler ResourceQuotaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ResourceQuotaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ResourceQuotaHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ResourceQuotaInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Get(name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Update(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ResourceQuotaList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ResourceQuotaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ResourceQuotaController
	AddHandler(ctx context.Context, name string, sync ResourceQuotaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ResourceQuotaHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ResourceQuotaLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ResourceQuotaLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ResourceQuotaLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ResourceQuotaLifecycle)
}

type resourceQuotaLister struct {
	ns         string
	controller *resourceQuotaController
}

func (l *resourceQuotaLister) List(namespace string, selector labels.Selector) (ret []*v1.ResourceQuota, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ResourceQuota))
	})
	return
}

func (l *resourceQuotaLister) Get(namespace, name string) (*v1.ResourceQuota, error) {
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
			Group:    ResourceQuotaGroupVersionKind.Group,
			Resource: ResourceQuotaGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ResourceQuota), nil
}

type resourceQuotaController struct {
	ns string
	controller.GenericController
}

func (c *resourceQuotaController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *resourceQuotaController) Lister() ResourceQuotaLister {
	return &resourceQuotaLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *resourceQuotaController) AddHandler(ctx context.Context, name string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *resourceQuotaController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *resourceQuotaController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *resourceQuotaController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type resourceQuotaFactory struct {
}

func (c resourceQuotaFactory) Object() runtime.Object {
	return &v1.ResourceQuota{}
}

func (c resourceQuotaFactory) List() runtime.Object {
	return &v1.ResourceQuotaList{}
}

func (s *resourceQuotaClient) Controller() ResourceQuotaController {
	genericController := controller.NewGenericController(s.ns, ResourceQuotaGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ResourceQuotaGroupVersionResource, ResourceQuotaGroupVersionKind.Kind, true))

	return &resourceQuotaController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type resourceQuotaClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ResourceQuotaController
}

func (s *resourceQuotaClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *resourceQuotaClient) Create(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Get(name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Update(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) UpdateStatus(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *resourceQuotaClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *resourceQuotaClient) List(opts metav1.ListOptions) (*v1.ResourceQuotaList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ResourceQuotaList), err
}

func (s *resourceQuotaClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ResourceQuotaList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ResourceQuotaList), err
}

func (s *resourceQuotaClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *resourceQuotaClient) Patch(o *v1.ResourceQuota, patchType types.PatchType, data []byte, subresources ...string) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *resourceQuotaClient) AddHandler(ctx context.Context, name string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *resourceQuotaClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *resourceQuotaClient) AddLifecycle(ctx context.Context, name string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *resourceQuotaClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *resourceQuotaClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
