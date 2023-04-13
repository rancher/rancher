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
	MultiClusterAppRevisionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "MultiClusterAppRevision",
	}
	MultiClusterAppRevisionResource = metav1.APIResource{
		Name:         "multiclusterapprevisions",
		SingularName: "multiclusterapprevision",
		Namespaced:   true,

		Kind: MultiClusterAppRevisionGroupVersionKind.Kind,
	}

	MultiClusterAppRevisionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "multiclusterapprevisions",
	}
)

func init() {
	resource.Put(MultiClusterAppRevisionGroupVersionResource)
}

// Deprecated: use v3.MultiClusterAppRevision instead
type MultiClusterAppRevision = v3.MultiClusterAppRevision

func NewMultiClusterAppRevision(namespace, name string, obj v3.MultiClusterAppRevision) *v3.MultiClusterAppRevision {
	obj.APIVersion, obj.Kind = MultiClusterAppRevisionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type MultiClusterAppRevisionHandlerFunc func(key string, obj *v3.MultiClusterAppRevision) (runtime.Object, error)

type MultiClusterAppRevisionChangeHandlerFunc func(obj *v3.MultiClusterAppRevision) (runtime.Object, error)

type MultiClusterAppRevisionLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.MultiClusterAppRevision, err error)
	Get(namespace, name string) (*v3.MultiClusterAppRevision, error)
}

type MultiClusterAppRevisionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() MultiClusterAppRevisionLister
	AddHandler(ctx context.Context, name string, handler MultiClusterAppRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler MultiClusterAppRevisionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type MultiClusterAppRevisionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.MultiClusterAppRevision) (*v3.MultiClusterAppRevision, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.MultiClusterAppRevision, error)
	Get(name string, opts metav1.GetOptions) (*v3.MultiClusterAppRevision, error)
	Update(*v3.MultiClusterAppRevision) (*v3.MultiClusterAppRevision, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.MultiClusterAppRevisionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.MultiClusterAppRevisionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MultiClusterAppRevisionController
	AddHandler(ctx context.Context, name string, sync MultiClusterAppRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppRevisionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppRevisionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppRevisionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle)
}

type multiClusterAppRevisionLister struct {
	ns         string
	controller *multiClusterAppRevisionController
}

func (l *multiClusterAppRevisionLister) List(namespace string, selector labels.Selector) (ret []*v3.MultiClusterAppRevision, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.MultiClusterAppRevision))
	})
	return
}

func (l *multiClusterAppRevisionLister) Get(namespace, name string) (*v3.MultiClusterAppRevision, error) {
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
			Group:    MultiClusterAppRevisionGroupVersionKind.Group,
			Resource: MultiClusterAppRevisionGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.MultiClusterAppRevision), nil
}

type multiClusterAppRevisionController struct {
	ns string
	controller.GenericController
}

func (c *multiClusterAppRevisionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *multiClusterAppRevisionController) Lister() MultiClusterAppRevisionLister {
	return &multiClusterAppRevisionLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *multiClusterAppRevisionController) AddHandler(ctx context.Context, name string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterAppRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppRevisionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterAppRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppRevisionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterAppRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppRevisionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.MultiClusterAppRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type multiClusterAppRevisionFactory struct {
}

func (c multiClusterAppRevisionFactory) Object() runtime.Object {
	return &v3.MultiClusterAppRevision{}
}

func (c multiClusterAppRevisionFactory) List() runtime.Object {
	return &v3.MultiClusterAppRevisionList{}
}

func (s *multiClusterAppRevisionClient) Controller() MultiClusterAppRevisionController {
	genericController := controller.NewGenericController(s.ns, MultiClusterAppRevisionGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(MultiClusterAppRevisionGroupVersionResource, MultiClusterAppRevisionGroupVersionKind.Kind, true))

	return &multiClusterAppRevisionController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type multiClusterAppRevisionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   MultiClusterAppRevisionController
}

func (s *multiClusterAppRevisionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *multiClusterAppRevisionClient) Create(o *v3.MultiClusterAppRevision) (*v3.MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) Get(name string, opts metav1.GetOptions) (*v3.MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.MultiClusterAppRevision, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) Update(o *v3.MultiClusterAppRevision) (*v3.MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) UpdateStatus(o *v3.MultiClusterAppRevision) (*v3.MultiClusterAppRevision, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *multiClusterAppRevisionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *multiClusterAppRevisionClient) List(opts metav1.ListOptions) (*v3.MultiClusterAppRevisionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.MultiClusterAppRevisionList), err
}

func (s *multiClusterAppRevisionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.MultiClusterAppRevisionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.MultiClusterAppRevisionList), err
}

func (s *multiClusterAppRevisionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *multiClusterAppRevisionClient) Patch(o *v3.MultiClusterAppRevision, patchType types.PatchType, data []byte, subresources ...string) (*v3.MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *multiClusterAppRevisionClient) AddHandler(ctx context.Context, name string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppRevisionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppRevisionClient) AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppRevisionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
