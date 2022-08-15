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

	GroupMemberGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "groupmembers",
	}
)

func init() {
	resource.Put(GroupMemberGroupVersionResource)
}

// Deprecated: use v3.GroupMember instead
type GroupMember = v3.GroupMember

func NewGroupMember(namespace, name string, obj v3.GroupMember) *v3.GroupMember {
	obj.APIVersion, obj.Kind = GroupMemberGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GroupMemberHandlerFunc func(key string, obj *v3.GroupMember) (runtime.Object, error)

type GroupMemberChangeHandlerFunc func(obj *v3.GroupMember) (runtime.Object, error)

type GroupMemberLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.GroupMember, err error)
	Get(namespace, name string) (*v3.GroupMember, error)
}

type GroupMemberController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GroupMemberLister
	AddHandler(ctx context.Context, name string, handler GroupMemberHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GroupMemberHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GroupMemberHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GroupMemberHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type GroupMemberInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.GroupMember) (*v3.GroupMember, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GroupMember, error)
	Get(name string, opts metav1.GetOptions) (*v3.GroupMember, error)
	Update(*v3.GroupMember) (*v3.GroupMember, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.GroupMemberList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GroupMemberList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GroupMemberController
	AddHandler(ctx context.Context, name string, sync GroupMemberHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GroupMemberHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GroupMemberLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GroupMemberLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GroupMemberHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GroupMemberHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GroupMemberLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GroupMemberLifecycle)
}

type groupMemberLister struct {
	ns         string
	controller *groupMemberController
}

func (l *groupMemberLister) List(namespace string, selector labels.Selector) (ret []*v3.GroupMember, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.GroupMember))
	})
	return
}

func (l *groupMemberLister) Get(namespace, name string) (*v3.GroupMember, error) {
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
			Resource: GroupMemberGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.GroupMember), nil
}

type groupMemberController struct {
	ns string
	controller.GenericController
}

func (c *groupMemberController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *groupMemberController) Lister() GroupMemberLister {
	return &groupMemberLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *groupMemberController) AddHandler(ctx context.Context, name string, handler GroupMemberHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GroupMember); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupMemberController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GroupMemberHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GroupMember); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupMemberController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GroupMemberHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GroupMember); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupMemberController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GroupMemberHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GroupMember); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type groupMemberFactory struct {
}

func (c groupMemberFactory) Object() runtime.Object {
	return &v3.GroupMember{}
}

func (c groupMemberFactory) List() runtime.Object {
	return &v3.GroupMemberList{}
}

func (s *groupMemberClient) Controller() GroupMemberController {
	genericController := controller.NewGenericController(s.ns, GroupMemberGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(GroupMemberGroupVersionResource, GroupMemberGroupVersionKind.Kind, false))

	return &groupMemberController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type groupMemberClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GroupMemberController
}

func (s *groupMemberClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *groupMemberClient) Create(o *v3.GroupMember) (*v3.GroupMember, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.GroupMember), err
}

func (s *groupMemberClient) Get(name string, opts metav1.GetOptions) (*v3.GroupMember, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.GroupMember), err
}

func (s *groupMemberClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GroupMember, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.GroupMember), err
}

func (s *groupMemberClient) Update(o *v3.GroupMember) (*v3.GroupMember, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.GroupMember), err
}

func (s *groupMemberClient) UpdateStatus(o *v3.GroupMember) (*v3.GroupMember, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.GroupMember), err
}

func (s *groupMemberClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *groupMemberClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *groupMemberClient) List(opts metav1.ListOptions) (*v3.GroupMemberList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.GroupMemberList), err
}

func (s *groupMemberClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GroupMemberList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.GroupMemberList), err
}

func (s *groupMemberClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *groupMemberClient) Patch(o *v3.GroupMember, patchType types.PatchType, data []byte, subresources ...string) (*v3.GroupMember, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.GroupMember), err
}

func (s *groupMemberClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *groupMemberClient) AddHandler(ctx context.Context, name string, sync GroupMemberHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *groupMemberClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GroupMemberHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *groupMemberClient) AddLifecycle(ctx context.Context, name string, lifecycle GroupMemberLifecycle) {
	sync := NewGroupMemberLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *groupMemberClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GroupMemberLifecycle) {
	sync := NewGroupMemberLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *groupMemberClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GroupMemberHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *groupMemberClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GroupMemberHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *groupMemberClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GroupMemberLifecycle) {
	sync := NewGroupMemberLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *groupMemberClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GroupMemberLifecycle) {
	sync := NewGroupMemberLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
