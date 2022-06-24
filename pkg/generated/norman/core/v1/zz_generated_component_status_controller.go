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
	ComponentStatusGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ComponentStatus",
	}
	ComponentStatusResource = metav1.APIResource{
		Name:         "componentstatuses",
		SingularName: "componentstatus",
		Namespaced:   false,
		Kind:         ComponentStatusGroupVersionKind.Kind,
	}

	ComponentStatusGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "componentstatuses",
	}
)

func init() {
	resource.Put(ComponentStatusGroupVersionResource)
}

// Deprecated: use v1.ComponentStatus instead
type ComponentStatus = v1.ComponentStatus

func NewComponentStatus(namespace, name string, obj v1.ComponentStatus) *v1.ComponentStatus {
	obj.APIVersion, obj.Kind = ComponentStatusGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ComponentStatusHandlerFunc func(key string, obj *v1.ComponentStatus) (runtime.Object, error)

type ComponentStatusChangeHandlerFunc func(obj *v1.ComponentStatus) (runtime.Object, error)

type ComponentStatusLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ComponentStatus, err error)
	Get(namespace, name string) (*v1.ComponentStatus, error)
}

type ComponentStatusController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ComponentStatusLister
	AddHandler(ctx context.Context, name string, handler ComponentStatusHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ComponentStatusHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ComponentStatusHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ComponentStatusHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ComponentStatusInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ComponentStatus) (*v1.ComponentStatus, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ComponentStatus, error)
	Get(name string, opts metav1.GetOptions) (*v1.ComponentStatus, error)
	Update(*v1.ComponentStatus) (*v1.ComponentStatus, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ComponentStatusList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ComponentStatusList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ComponentStatusController
	AddHandler(ctx context.Context, name string, sync ComponentStatusHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ComponentStatusHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ComponentStatusLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ComponentStatusLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ComponentStatusHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ComponentStatusHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ComponentStatusLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ComponentStatusLifecycle)
}

type componentStatusLister struct {
	ns         string
	controller *componentStatusController
}

func (l *componentStatusLister) List(namespace string, selector labels.Selector) (ret []*v1.ComponentStatus, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ComponentStatus))
	})
	return
}

func (l *componentStatusLister) Get(namespace, name string) (*v1.ComponentStatus, error) {
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
			Group:    ComponentStatusGroupVersionKind.Group,
			Resource: ComponentStatusGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ComponentStatus), nil
}

type componentStatusController struct {
	ns string
	controller.GenericController
}

func (c *componentStatusController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *componentStatusController) Lister() ComponentStatusLister {
	return &componentStatusLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *componentStatusController) AddHandler(ctx context.Context, name string, handler ComponentStatusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ComponentStatus); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *componentStatusController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ComponentStatusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ComponentStatus); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *componentStatusController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ComponentStatusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ComponentStatus); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *componentStatusController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ComponentStatusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ComponentStatus); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type componentStatusFactory struct {
}

func (c componentStatusFactory) Object() runtime.Object {
	return &v1.ComponentStatus{}
}

func (c componentStatusFactory) List() runtime.Object {
	return &v1.ComponentStatusList{}
}

func (s *componentStatusClient) Controller() ComponentStatusController {
	genericController := controller.NewGenericController(s.ns, ComponentStatusGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ComponentStatusGroupVersionResource, ComponentStatusGroupVersionKind.Kind, false))

	return &componentStatusController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type componentStatusClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ComponentStatusController
}

func (s *componentStatusClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *componentStatusClient) Create(o *v1.ComponentStatus) (*v1.ComponentStatus, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ComponentStatus), err
}

func (s *componentStatusClient) Get(name string, opts metav1.GetOptions) (*v1.ComponentStatus, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ComponentStatus), err
}

func (s *componentStatusClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ComponentStatus, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ComponentStatus), err
}

func (s *componentStatusClient) Update(o *v1.ComponentStatus) (*v1.ComponentStatus, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ComponentStatus), err
}

func (s *componentStatusClient) UpdateStatus(o *v1.ComponentStatus) (*v1.ComponentStatus, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ComponentStatus), err
}

func (s *componentStatusClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *componentStatusClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *componentStatusClient) List(opts metav1.ListOptions) (*v1.ComponentStatusList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ComponentStatusList), err
}

func (s *componentStatusClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ComponentStatusList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ComponentStatusList), err
}

func (s *componentStatusClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *componentStatusClient) Patch(o *v1.ComponentStatus, patchType types.PatchType, data []byte, subresources ...string) (*v1.ComponentStatus, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ComponentStatus), err
}

func (s *componentStatusClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *componentStatusClient) AddHandler(ctx context.Context, name string, sync ComponentStatusHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *componentStatusClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ComponentStatusHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *componentStatusClient) AddLifecycle(ctx context.Context, name string, lifecycle ComponentStatusLifecycle) {
	sync := NewComponentStatusLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *componentStatusClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ComponentStatusLifecycle) {
	sync := NewComponentStatusLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *componentStatusClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ComponentStatusHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *componentStatusClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ComponentStatusHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *componentStatusClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ComponentStatusLifecycle) {
	sync := NewComponentStatusLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *componentStatusClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ComponentStatusLifecycle) {
	sync := NewComponentStatusLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
