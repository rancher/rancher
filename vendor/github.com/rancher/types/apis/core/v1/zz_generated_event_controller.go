package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type EventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Event
}

type EventHandlerFunc func(key string, obj *v1.Event) error

type EventLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Event, err error)
	Get(namespace, name string) (*v1.Event, error)
}

type EventController interface {
	Informer() cache.SharedIndexInformer
	Lister() EventLister
	AddHandler(handler EventHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type EventInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.Event) (*v1.Event, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Event, error)
	Get(name string, opts metav1.GetOptions) (*v1.Event, error)
	Update(*v1.Event) (*v1.Event, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*EventList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() EventController
	AddSyncHandler(sync EventHandlerFunc)
	AddLifecycle(name string, lifecycle EventLifecycle)
}

type eventLister struct {
	controller *eventController
}

func (l *eventLister) List(namespace string, selector labels.Selector) (ret []*v1.Event, err error) {
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
			Resource: "event",
		}, name)
	}
	return obj.(*v1.Event), nil
}

type eventController struct {
	controller.GenericController
}

func (c *eventController) Lister() EventLister {
	return &eventLister{
		controller: c,
	}
}

func (c *eventController) AddHandler(handler EventHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Event))
	})
}

type eventFactory struct {
}

func (c eventFactory) Object() runtime.Object {
	return &v1.Event{}
}

func (c eventFactory) List() runtime.Object {
	return &EventList{}
}

func (s *eventClient) Controller() EventController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.eventControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(EventGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &eventController{
		GenericController: genericController,
	}

	s.client.eventControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type eventClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   EventController
}

func (s *eventClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *eventClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Event, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*v1.Event), err
}

func (s *eventClient) Update(o *v1.Event) (*v1.Event, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Event), err
}

func (s *eventClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *eventClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *eventClient) List(opts metav1.ListOptions) (*EventList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*EventList), err
}

func (s *eventClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *eventClient) Patch(o *v1.Event, data []byte, subresources ...string) (*v1.Event, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.Event), err
}

func (s *eventClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *eventClient) AddSyncHandler(sync EventHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *eventClient) AddLifecycle(name string, lifecycle EventLifecycle) {
	sync := NewEventLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
