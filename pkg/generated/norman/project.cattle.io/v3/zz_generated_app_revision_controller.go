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
	AppRevisionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "AppRevision",
	}
	AppRevisionResource = metav1.APIResource{
		Name:         "apprevisions",
		SingularName: "apprevision",
		Namespaced:   true,

		Kind: AppRevisionGroupVersionKind.Kind,
	}

	AppRevisionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "apprevisions",
	}
)

func init() {
	resource.Put(AppRevisionGroupVersionResource)
}

// Deprecated: use v3.AppRevision instead
type AppRevision = v3.AppRevision

func NewAppRevision(namespace, name string, obj v3.AppRevision) *v3.AppRevision {
	obj.APIVersion, obj.Kind = AppRevisionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AppRevisionHandlerFunc func(key string, obj *v3.AppRevision) (runtime.Object, error)

type AppRevisionChangeHandlerFunc func(obj *v3.AppRevision) (runtime.Object, error)

type AppRevisionLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.AppRevision, err error)
	Get(namespace, name string) (*v3.AppRevision, error)
}

type AppRevisionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AppRevisionLister
	AddHandler(ctx context.Context, name string, handler AppRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AppRevisionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AppRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler AppRevisionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type AppRevisionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.AppRevision) (*v3.AppRevision, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.AppRevision, error)
	Get(name string, opts metav1.GetOptions) (*v3.AppRevision, error)
	Update(*v3.AppRevision) (*v3.AppRevision, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.AppRevisionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.AppRevisionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AppRevisionController
	AddHandler(ctx context.Context, name string, sync AppRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AppRevisionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AppRevisionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AppRevisionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AppRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AppRevisionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AppRevisionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AppRevisionLifecycle)
}

type appRevisionLister struct {
	ns         string
	controller *appRevisionController
}

func (l *appRevisionLister) List(namespace string, selector labels.Selector) (ret []*v3.AppRevision, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.AppRevision))
	})
	return
}

func (l *appRevisionLister) Get(namespace, name string) (*v3.AppRevision, error) {
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
			Group:    AppRevisionGroupVersionKind.Group,
			Resource: AppRevisionGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.AppRevision), nil
}

type appRevisionController struct {
	ns string
	controller.GenericController
}

func (c *appRevisionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *appRevisionController) Lister() AppRevisionLister {
	return &appRevisionLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *appRevisionController) AddHandler(ctx context.Context, name string, handler AppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.AppRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *appRevisionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler AppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.AppRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *appRevisionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.AppRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *appRevisionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler AppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.AppRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type appRevisionFactory struct {
}

func (c appRevisionFactory) Object() runtime.Object {
	return &v3.AppRevision{}
}

func (c appRevisionFactory) List() runtime.Object {
	return &v3.AppRevisionList{}
}

func (s *appRevisionClient) Controller() AppRevisionController {
	genericController := controller.NewGenericController(s.ns, AppRevisionGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(AppRevisionGroupVersionResource, AppRevisionGroupVersionKind.Kind, true))

	return &appRevisionController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type appRevisionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AppRevisionController
}

func (s *appRevisionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *appRevisionClient) Create(o *v3.AppRevision) (*v3.AppRevision, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.AppRevision), err
}

func (s *appRevisionClient) Get(name string, opts metav1.GetOptions) (*v3.AppRevision, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.AppRevision), err
}

func (s *appRevisionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.AppRevision, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.AppRevision), err
}

func (s *appRevisionClient) Update(o *v3.AppRevision) (*v3.AppRevision, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.AppRevision), err
}

func (s *appRevisionClient) UpdateStatus(o *v3.AppRevision) (*v3.AppRevision, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.AppRevision), err
}

func (s *appRevisionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *appRevisionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *appRevisionClient) List(opts metav1.ListOptions) (*v3.AppRevisionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.AppRevisionList), err
}

func (s *appRevisionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.AppRevisionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.AppRevisionList), err
}

func (s *appRevisionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *appRevisionClient) Patch(o *v3.AppRevision, patchType types.PatchType, data []byte, subresources ...string) (*v3.AppRevision, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.AppRevision), err
}

func (s *appRevisionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *appRevisionClient) AddHandler(ctx context.Context, name string, sync AppRevisionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *appRevisionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AppRevisionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *appRevisionClient) AddLifecycle(ctx context.Context, name string, lifecycle AppRevisionLifecycle) {
	sync := NewAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *appRevisionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AppRevisionLifecycle) {
	sync := NewAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *appRevisionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *appRevisionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *appRevisionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AppRevisionLifecycle) {
	sync := NewAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *appRevisionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AppRevisionLifecycle) {
	sync := NewAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
