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
	LimitRangeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "LimitRange",
	}
	LimitRangeResource = metav1.APIResource{
		Name:         "limitranges",
		SingularName: "limitrange",
		Namespaced:   true,

		Kind: LimitRangeGroupVersionKind.Kind,
	}

	LimitRangeGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "limitranges",
	}
)

func init() {
	resource.Put(LimitRangeGroupVersionResource)
}

// Deprecated: use v1.LimitRange instead
type LimitRange = v1.LimitRange

func NewLimitRange(namespace, name string, obj v1.LimitRange) *v1.LimitRange {
	obj.APIVersion, obj.Kind = LimitRangeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type LimitRangeHandlerFunc func(key string, obj *v1.LimitRange) (runtime.Object, error)

type LimitRangeChangeHandlerFunc func(obj *v1.LimitRange) (runtime.Object, error)

type LimitRangeLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.LimitRange, err error)
	Get(namespace, name string) (*v1.LimitRange, error)
}

type LimitRangeController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() LimitRangeLister
	AddHandler(ctx context.Context, name string, handler LimitRangeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync LimitRangeHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler LimitRangeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler LimitRangeHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type LimitRangeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.LimitRange) (*v1.LimitRange, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.LimitRange, error)
	Get(name string, opts metav1.GetOptions) (*v1.LimitRange, error)
	Update(*v1.LimitRange) (*v1.LimitRange, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.LimitRangeList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.LimitRangeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() LimitRangeController
	AddHandler(ctx context.Context, name string, sync LimitRangeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync LimitRangeHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle LimitRangeLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle LimitRangeLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync LimitRangeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync LimitRangeHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle LimitRangeLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle LimitRangeLifecycle)
}

type limitRangeLister struct {
	ns         string
	controller *limitRangeController
}

func (l *limitRangeLister) List(namespace string, selector labels.Selector) (ret []*v1.LimitRange, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.LimitRange))
	})
	return
}

func (l *limitRangeLister) Get(namespace, name string) (*v1.LimitRange, error) {
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
			Group:    LimitRangeGroupVersionKind.Group,
			Resource: LimitRangeGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.LimitRange), nil
}

type limitRangeController struct {
	ns string
	controller.GenericController
}

func (c *limitRangeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *limitRangeController) Lister() LimitRangeLister {
	return &limitRangeLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *limitRangeController) AddHandler(ctx context.Context, name string, handler LimitRangeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.LimitRange); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *limitRangeController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler LimitRangeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.LimitRange); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *limitRangeController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler LimitRangeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.LimitRange); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *limitRangeController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler LimitRangeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.LimitRange); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type limitRangeFactory struct {
}

func (c limitRangeFactory) Object() runtime.Object {
	return &v1.LimitRange{}
}

func (c limitRangeFactory) List() runtime.Object {
	return &v1.LimitRangeList{}
}

func (s *limitRangeClient) Controller() LimitRangeController {
	genericController := controller.NewGenericController(s.ns, LimitRangeGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(LimitRangeGroupVersionResource, LimitRangeGroupVersionKind.Kind, true))

	return &limitRangeController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type limitRangeClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   LimitRangeController
}

func (s *limitRangeClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *limitRangeClient) Create(o *v1.LimitRange) (*v1.LimitRange, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.LimitRange), err
}

func (s *limitRangeClient) Get(name string, opts metav1.GetOptions) (*v1.LimitRange, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.LimitRange), err
}

func (s *limitRangeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.LimitRange, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.LimitRange), err
}

func (s *limitRangeClient) Update(o *v1.LimitRange) (*v1.LimitRange, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.LimitRange), err
}

func (s *limitRangeClient) UpdateStatus(o *v1.LimitRange) (*v1.LimitRange, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.LimitRange), err
}

func (s *limitRangeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *limitRangeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *limitRangeClient) List(opts metav1.ListOptions) (*v1.LimitRangeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.LimitRangeList), err
}

func (s *limitRangeClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.LimitRangeList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.LimitRangeList), err
}

func (s *limitRangeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *limitRangeClient) Patch(o *v1.LimitRange, patchType types.PatchType, data []byte, subresources ...string) (*v1.LimitRange, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.LimitRange), err
}

func (s *limitRangeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *limitRangeClient) AddHandler(ctx context.Context, name string, sync LimitRangeHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *limitRangeClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync LimitRangeHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *limitRangeClient) AddLifecycle(ctx context.Context, name string, lifecycle LimitRangeLifecycle) {
	sync := NewLimitRangeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *limitRangeClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle LimitRangeLifecycle) {
	sync := NewLimitRangeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *limitRangeClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync LimitRangeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *limitRangeClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync LimitRangeHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *limitRangeClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle LimitRangeLifecycle) {
	sync := NewLimitRangeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *limitRangeClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle LimitRangeLifecycle) {
	sync := NewLimitRangeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
