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
	GroupGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Group",
	}
	GroupResource = metav1.APIResource{
		Name:         "groups",
		SingularName: "group",
		Namespaced:   false,
		Kind:         GroupGroupVersionKind.Kind,
	}
)

type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group
}

type GroupHandlerFunc func(key string, obj *Group) error

type GroupLister interface {
	List(namespace string, selector labels.Selector) (ret []*Group, err error)
	Get(namespace, name string) (*Group, error)
}

type GroupController interface {
	Informer() cache.SharedIndexInformer
	Lister() GroupLister
	AddHandler(name string, handler GroupHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GroupInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Group) (*Group, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*Group, error)
	Get(name string, opts metav1.GetOptions) (*Group, error)
	Update(*Group) (*Group, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GroupController
	AddHandler(name string, sync GroupHandlerFunc)
	AddLifecycle(name string, lifecycle GroupLifecycle)
}

type groupLister struct {
	controller *groupController
}

func (l *groupLister) List(namespace string, selector labels.Selector) (ret []*Group, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Group))
	})
	return
}

func (l *groupLister) Get(namespace, name string) (*Group, error) {
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
			Group:    GroupGroupVersionKind.Group,
			Resource: "group",
		}, name)
	}
	return obj.(*Group), nil
}

type groupController struct {
	controller.GenericController
}

func (c *groupController) Lister() GroupLister {
	return &groupLister{
		controller: c,
	}
}

func (c *groupController) AddHandler(name string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Group))
	})
}

type groupFactory struct {
}

func (c groupFactory) Object() runtime.Object {
	return &Group{}
}

func (c groupFactory) List() runtime.Object {
	return &GroupList{}
}

func (s *groupClient) Controller() GroupController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.groupControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GroupGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &groupController{
		GenericController: genericController,
	}

	s.client.groupControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type groupClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   GroupController
}

func (s *groupClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *groupClient) Create(o *Group) (*Group, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Group), err
}

func (s *groupClient) Get(name string, opts metav1.GetOptions) (*Group, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Group), err
}

func (s *groupClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*Group, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*Group), err
}

func (s *groupClient) Update(o *Group) (*Group, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Group), err
}

func (s *groupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *groupClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *groupClient) List(opts metav1.ListOptions) (*GroupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GroupList), err
}

func (s *groupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *groupClient) Patch(o *Group, data []byte, subresources ...string) (*Group, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Group), err
}

func (s *groupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *groupClient) AddHandler(name string, sync GroupHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *groupClient) AddLifecycle(name string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name, s, lifecycle)
	s.AddHandler(name, sync)
}
