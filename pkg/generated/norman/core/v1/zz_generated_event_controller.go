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
	EventGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Event",
	}
	EventResource = metav1.APIResource{
		Name:         "events",
		SingularName: "event",
		Namespaced:   false,
		Kind:         EventGroupVersionKind.Kind,
	}

	EventGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "events",
	}
)

func init() {
	resource.Put(EventGroupVersionResource)
}

// Deprecated: use v1.Event instead
type Event = v1.Event

func NewEvent(namespace, name string, obj v1.Event) *v1.Event {
	obj.APIVersion, obj.Kind = EventGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type EventHandlerFunc func(key string, obj *v1.Event) (runtime.Object, error)

type EventChangeHandlerFunc func(obj *v1.Event) (runtime.Object, error)

type EventLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Event, err error)
	Get(namespace, name string) (*v1.Event, error)
}

type EventController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() EventLister
	AddHandler(ctx context.Context, name string, handler EventHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EventHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler EventHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler EventHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type EventInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Event) (*v1.Event, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Event, error)
	Get(name string, opts metav1.GetOptions) (*v1.Event, error)
	Update(*v1.Event) (*v1.Event, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.EventList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.EventList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() EventController
	AddHandler(ctx context.Context, name string, sync EventHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EventHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle EventLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EventLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EventHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EventHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EventLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EventLifecycle)
}

type eventLister struct {
	ns         string
	controller *eventController
}

func (l *eventLister) List(namespace string, selector labels.Selector) (ret []*v1.Event, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Event))
	})
	return
}

func (l *eventLister) Get(namespace, name string) (*v1.Event, error) {
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
			Group:    EventGroupVersionKind.Group,
			Resource: EventGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Event), nil
}

type eventController struct {
	ns string
	controller.GenericController
}

func (c *eventController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *eventController) Lister() EventLister {
	return &eventLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *eventController) AddHandler(ctx context.Context, name string, handler EventHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Event); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *eventController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler EventHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Event); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *eventController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler EventHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Event); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *eventController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler EventHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Event); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type eventFactory struct {
}

func (c eventFactory) Object() runtime.Object {
	return &v1.Event{}
}

func (c eventFactory) List() runtime.Object {
	return &v1.EventList{}
}

func (s *eventClient) Controller() EventController {
	genericController := controller.NewGenericController(s.ns, EventGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(EventGroupVersionResource, EventGroupVersionKind.Kind, false))

	return &eventController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type eventClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   EventController
}

func (s *eventClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *eventClient) Create(o *v1.Event) (*v1.Event, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Event), err
}

func (s *eventClient) Get(name string, opts metav1.GetOptions) (*v1.Event, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Event), err
}

func (s *eventClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Event, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Event), err
}

func (s *eventClient) Update(o *v1.Event) (*v1.Event, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Event), err
}

func (s *eventClient) UpdateStatus(o *v1.Event) (*v1.Event, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Event), err
}

func (s *eventClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *eventClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *eventClient) List(opts metav1.ListOptions) (*v1.EventList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.EventList), err
}

func (s *eventClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.EventList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.EventList), err
}

func (s *eventClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *eventClient) Patch(o *v1.Event, patchType types.PatchType, data []byte, subresources ...string) (*v1.Event, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Event), err
}

func (s *eventClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *eventClient) AddHandler(ctx context.Context, name string, sync EventHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *eventClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EventHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *eventClient) AddLifecycle(ctx context.Context, name string, lifecycle EventLifecycle) {
	sync := NewEventLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *eventClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EventLifecycle) {
	sync := NewEventLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *eventClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EventHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *eventClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EventHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *eventClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EventLifecycle) {
	sync := NewEventLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *eventClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EventLifecycle) {
	sync := NewEventLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
