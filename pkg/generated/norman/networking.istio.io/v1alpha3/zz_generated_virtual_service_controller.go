package v1alpha3

import (
	"context"
	"time"

	"github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
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
	VirtualServiceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "VirtualService",
	}
	VirtualServiceResource = metav1.APIResource{
		Name:         "virtualservices",
		SingularName: "virtualservice",
		Namespaced:   true,

		Kind: VirtualServiceGroupVersionKind.Kind,
	}

	VirtualServiceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "virtualservices",
	}
)

func init() {
	resource.Put(VirtualServiceGroupVersionResource)
}

// Deprecated use v1alpha3.VirtualService instead
type VirtualService = v1alpha3.VirtualService

func NewVirtualService(namespace, name string, obj v1alpha3.VirtualService) *v1alpha3.VirtualService {
	obj.APIVersion, obj.Kind = VirtualServiceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type VirtualServiceHandlerFunc func(key string, obj *v1alpha3.VirtualService) (runtime.Object, error)

type VirtualServiceChangeHandlerFunc func(obj *v1alpha3.VirtualService) (runtime.Object, error)

type VirtualServiceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1alpha3.VirtualService, err error)
	Get(namespace, name string) (*v1alpha3.VirtualService, error)
}

type VirtualServiceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() VirtualServiceLister
	AddHandler(ctx context.Context, name string, handler VirtualServiceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync VirtualServiceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler VirtualServiceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler VirtualServiceHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type VirtualServiceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1alpha3.VirtualService) (*v1alpha3.VirtualService, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1alpha3.VirtualService, error)
	Get(name string, opts metav1.GetOptions) (*v1alpha3.VirtualService, error)
	Update(*v1alpha3.VirtualService) (*v1alpha3.VirtualService, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1alpha3.VirtualServiceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1alpha3.VirtualServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() VirtualServiceController
	AddHandler(ctx context.Context, name string, sync VirtualServiceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync VirtualServiceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle VirtualServiceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle VirtualServiceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync VirtualServiceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync VirtualServiceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle VirtualServiceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle VirtualServiceLifecycle)
}

type virtualServiceLister struct {
	controller *virtualServiceController
}

func (l *virtualServiceLister) List(namespace string, selector labels.Selector) (ret []*v1alpha3.VirtualService, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1alpha3.VirtualService))
	})
	return
}

func (l *virtualServiceLister) Get(namespace, name string) (*v1alpha3.VirtualService, error) {
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
			Group:    VirtualServiceGroupVersionKind.Group,
			Resource: VirtualServiceGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1alpha3.VirtualService), nil
}

type virtualServiceController struct {
	controller.GenericController
}

func (c *virtualServiceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *virtualServiceController) Lister() VirtualServiceLister {
	return &virtualServiceLister{
		controller: c,
	}
}

func (c *virtualServiceController) AddHandler(ctx context.Context, name string, handler VirtualServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.VirtualService); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *virtualServiceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler VirtualServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.VirtualService); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *virtualServiceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler VirtualServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.VirtualService); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *virtualServiceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler VirtualServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.VirtualService); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type virtualServiceFactory struct {
}

func (c virtualServiceFactory) Object() runtime.Object {
	return &v1alpha3.VirtualService{}
}

func (c virtualServiceFactory) List() runtime.Object {
	return &v1alpha3.VirtualServiceList{}
}

func (s *virtualServiceClient) Controller() VirtualServiceController {
	genericController := controller.NewGenericController(VirtualServiceGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(VirtualServiceGroupVersionResource, VirtualServiceGroupVersionKind.Kind, true))

	return &virtualServiceController{
		GenericController: genericController,
	}
}

type virtualServiceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   VirtualServiceController
}

func (s *virtualServiceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *virtualServiceClient) Create(o *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1alpha3.VirtualService), err
}

func (s *virtualServiceClient) Get(name string, opts metav1.GetOptions) (*v1alpha3.VirtualService, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1alpha3.VirtualService), err
}

func (s *virtualServiceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1alpha3.VirtualService, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1alpha3.VirtualService), err
}

func (s *virtualServiceClient) Update(o *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1alpha3.VirtualService), err
}

func (s *virtualServiceClient) UpdateStatus(o *v1alpha3.VirtualService) (*v1alpha3.VirtualService, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1alpha3.VirtualService), err
}

func (s *virtualServiceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *virtualServiceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *virtualServiceClient) List(opts metav1.ListOptions) (*v1alpha3.VirtualServiceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1alpha3.VirtualServiceList), err
}

func (s *virtualServiceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1alpha3.VirtualServiceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1alpha3.VirtualServiceList), err
}

func (s *virtualServiceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *virtualServiceClient) Patch(o *v1alpha3.VirtualService, patchType types.PatchType, data []byte, subresources ...string) (*v1alpha3.VirtualService, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1alpha3.VirtualService), err
}

func (s *virtualServiceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *virtualServiceClient) AddHandler(ctx context.Context, name string, sync VirtualServiceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *virtualServiceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync VirtualServiceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *virtualServiceClient) AddLifecycle(ctx context.Context, name string, lifecycle VirtualServiceLifecycle) {
	sync := NewVirtualServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *virtualServiceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle VirtualServiceLifecycle) {
	sync := NewVirtualServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *virtualServiceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync VirtualServiceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *virtualServiceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync VirtualServiceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *virtualServiceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle VirtualServiceLifecycle) {
	sync := NewVirtualServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *virtualServiceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle VirtualServiceLifecycle) {
	sync := NewVirtualServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
