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
	GroupMemberGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GroupMember",
	}
	GroupMemberResource = metav1.APIResource{
		Name:         "groupmembers",
		SingularName: "groupmember",
		Namespaced:   false,
		Kind:         GroupMemberGroupVersionKind.Kind,
	}
)

type GroupMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GroupMember
}

type GroupMemberHandlerFunc func(key string, obj *GroupMember) error

type GroupMemberLister interface {
	List(namespace string, selector labels.Selector) (ret []*GroupMember, err error)
	Get(namespace, name string) (*GroupMember, error)
}

type GroupMemberController interface {
	Informer() cache.SharedIndexInformer
	Lister() GroupMemberLister
	AddHandler(name string, handler GroupMemberHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler GroupMemberHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GroupMemberInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*GroupMember) (*GroupMember, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*GroupMember, error)
	Get(name string, opts metav1.GetOptions) (*GroupMember, error)
	Update(*GroupMember) (*GroupMember, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GroupMemberList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GroupMemberController
	AddHandler(name string, sync GroupMemberHandlerFunc)
	AddLifecycle(name string, lifecycle GroupMemberLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync GroupMemberHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle GroupMemberLifecycle)
}

type groupMemberLister struct {
	controller *groupMemberController
}

func (l *groupMemberLister) List(namespace string, selector labels.Selector) (ret []*GroupMember, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GroupMember))
	})
	return
}

func (l *groupMemberLister) Get(namespace, name string) (*GroupMember, error) {
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
			Group:    GroupMemberGroupVersionKind.Group,
			Resource: "groupMember",
		}, name)
	}
	return obj.(*GroupMember), nil
}

type groupMemberController struct {
	controller.GenericController
}

func (c *groupMemberController) Lister() GroupMemberLister {
	return &groupMemberLister{
		controller: c,
	}
}

func (c *groupMemberController) AddHandler(name string, handler GroupMemberHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*GroupMember))
	})
}

func (c *groupMemberController) AddClusterScopedHandler(name, cluster string, handler GroupMemberHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*GroupMember))
	})
}

type groupMemberFactory struct {
}

func (c groupMemberFactory) Object() runtime.Object {
	return &GroupMember{}
}

func (c groupMemberFactory) List() runtime.Object {
	return &GroupMemberList{}
}

func (s *groupMemberClient) Controller() GroupMemberController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.groupMemberControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GroupMemberGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &groupMemberController{
		GenericController: genericController,
	}

	s.client.groupMemberControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type groupMemberClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   GroupMemberController
}

func (s *groupMemberClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *groupMemberClient) Create(o *GroupMember) (*GroupMember, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GroupMember), err
}

func (s *groupMemberClient) Get(name string, opts metav1.GetOptions) (*GroupMember, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GroupMember), err
}

func (s *groupMemberClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*GroupMember, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*GroupMember), err
}

func (s *groupMemberClient) Update(o *GroupMember) (*GroupMember, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GroupMember), err
}

func (s *groupMemberClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *groupMemberClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *groupMemberClient) List(opts metav1.ListOptions) (*GroupMemberList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GroupMemberList), err
}

func (s *groupMemberClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *groupMemberClient) Patch(o *GroupMember, data []byte, subresources ...string) (*GroupMember, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*GroupMember), err
}

func (s *groupMemberClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *groupMemberClient) AddHandler(name string, sync GroupMemberHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *groupMemberClient) AddLifecycle(name string, lifecycle GroupMemberLifecycle) {
	sync := NewGroupMemberLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *groupMemberClient) AddClusterScopedHandler(name, clusterName string, sync GroupMemberHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *groupMemberClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle GroupMemberLifecycle) {
	sync := NewGroupMemberLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
