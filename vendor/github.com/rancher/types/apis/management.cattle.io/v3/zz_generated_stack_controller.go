package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	StackGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Stack",
	}
	StackResource = metav1.APIResource{
		Name:         "stacks",
		SingularName: "stack",
		Namespaced:   true,

		Kind: StackGroupVersionKind.Kind,
	}
)

type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stack
}

type StackHandlerFunc func(key string, obj *Stack) error

type StackLister interface {
	List(namespace string, selector labels.Selector) (ret []*Stack, err error)
	Get(namespace, name string) (*Stack, error)
}

type StackController interface {
	Informer() cache.SharedIndexInformer
	Lister() StackLister
	AddHandler(name string, handler StackHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type StackInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Stack) (*Stack, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*Stack, error)
	Get(name string, opts metav1.GetOptions) (*Stack, error)
	Update(*Stack) (*Stack, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*StackList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() StackController
	AddHandler(name string, sync StackHandlerFunc)
	AddLifecycle(name string, lifecycle StackLifecycle)
}

type stackLister struct {
	controller *stackController
}

func (l *stackLister) List(namespace string, selector labels.Selector) (ret []*Stack, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Stack))
	})
	return
}

func (l *stackLister) Get(namespace, name string) (*Stack, error) {
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
			Group:    StackGroupVersionKind.Group,
			Resource: "stack",
		}, name)
	}
	return obj.(*Stack), nil
}

type stackController struct {
	controller.GenericController
}

func (c *stackController) Lister() StackLister {
	return &stackLister{
		controller: c,
	}
}

func (c *stackController) AddHandler(name string, handler StackHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Stack))
	})
}

type stackFactory struct {
}

func (c stackFactory) Object() runtime.Object {
	return &Stack{}
}

func (c stackFactory) List() runtime.Object {
	return &StackList{}
}

func (s *stackClient) Controller() StackController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.stackControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(StackGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &stackController{
		GenericController: genericController,
	}

	s.client.stackControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type stackClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   StackController
}

func (s *stackClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *stackClient) Create(o *Stack) (*Stack, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Stack), err
}

func (s *stackClient) Get(name string, opts metav1.GetOptions) (*Stack, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Stack), err
}

func (s *stackClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*Stack, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*Stack), err
}

func (s *stackClient) Update(o *Stack) (*Stack, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Stack), err
}

func (s *stackClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *stackClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *stackClient) List(opts metav1.ListOptions) (*StackList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*StackList), err
}

func (s *stackClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *stackClient) Patch(o *Stack, data []byte, subresources ...string) (*Stack, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Stack), err
}

func (s *stackClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *stackClient) AddHandler(name string, sync StackHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *stackClient) AddLifecycle(name string, lifecycle StackLifecycle) {
	sync := NewStackLifecycleAdapter(name, s, lifecycle)
	s.AddHandler(name, sync)
}
