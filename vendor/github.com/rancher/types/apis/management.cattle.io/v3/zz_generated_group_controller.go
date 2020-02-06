package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
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

	GroupGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "groups",
	}
)

func init() {
	resource.Put(GroupGroupVersionResource)
}

func NewGroup(namespace, name string, obj Group) *Group {
	obj.APIVersion, obj.Kind = GroupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}

type GroupHandlerFunc func(key string, obj *Group) (runtime.Object, error)

type GroupChangeHandlerFunc func(obj *Group) (runtime.Object, error)

type GroupLister interface {
	List(namespace string, selector labels.Selector) (ret []*Group, err error)
	Get(namespace, name string) (*Group, error)
}

type GroupController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GroupLister
	AddHandler(ctx context.Context, name string, handler GroupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GroupHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GroupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GroupHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GroupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Group) (*Group, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Group, error)
	Get(name string, opts metav1.GetOptions) (*Group, error)
	Update(*Group) (*Group, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GroupList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*GroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GroupController
	AddHandler(ctx context.Context, name string, sync GroupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GroupHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GroupLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GroupLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GroupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GroupHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GroupLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GroupLifecycle)
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
		}, key)
	}
	return obj.(*Group), nil
}

type groupController struct {
	controller.GenericController
}

func (c *groupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *groupController) Lister() GroupLister {
	return &groupLister{
		controller: c,
	}
}

func (c *groupController) AddHandler(ctx context.Context, name string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Group); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Group); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Group); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Group); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
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
	objectClient *objectclient.ObjectClient
	controller   GroupController
}

func (s *groupClient) ObjectClient() *objectclient.ObjectClient {
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

func (s *groupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Group, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Group), err
}

func (s *groupClient) Update(o *Group) (*Group, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Group), err
}

func (s *groupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *groupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *groupClient) List(opts metav1.ListOptions) (*GroupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GroupList), err
}

func (s *groupClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*GroupList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*GroupList), err
}

func (s *groupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *groupClient) Patch(o *Group, patchType types.PatchType, data []byte, subresources ...string) (*Group, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Group), err
}

func (s *groupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *groupClient) AddHandler(ctx context.Context, name string, sync GroupHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *groupClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GroupHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *groupClient) AddLifecycle(ctx context.Context, name string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *groupClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *groupClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GroupHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *groupClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GroupHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *groupClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *groupClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
