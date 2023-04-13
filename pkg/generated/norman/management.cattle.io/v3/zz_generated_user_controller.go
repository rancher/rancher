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
	UserGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "User",
	}
	UserResource = metav1.APIResource{
		Name:         "users",
		SingularName: "user",
		Namespaced:   false,
		Kind:         UserGroupVersionKind.Kind,
	}

	UserGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "users",
	}
)

func init() {
	resource.Put(UserGroupVersionResource)
}

// Deprecated: use v3.User instead
type User = v3.User

func NewUser(namespace, name string, obj v3.User) *v3.User {
	obj.APIVersion, obj.Kind = UserGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type UserHandlerFunc func(key string, obj *v3.User) (runtime.Object, error)

type UserChangeHandlerFunc func(obj *v3.User) (runtime.Object, error)

type UserLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.User, err error)
	Get(namespace, name string) (*v3.User, error)
}

type UserController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() UserLister
	AddHandler(ctx context.Context, name string, handler UserHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler UserHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler UserHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type UserInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.User) (*v3.User, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.User, error)
	Get(name string, opts metav1.GetOptions) (*v3.User, error)
	Update(*v3.User) (*v3.User, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.UserList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.UserList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() UserController
	AddHandler(ctx context.Context, name string, sync UserHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle UserLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle UserLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync UserHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle UserLifecycle)
}

type userLister struct {
	ns         string
	controller *userController
}

func (l *userLister) List(namespace string, selector labels.Selector) (ret []*v3.User, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.User))
	})
	return
}

func (l *userLister) Get(namespace, name string) (*v3.User, error) {
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
			Group:    UserGroupVersionKind.Group,
			Resource: UserGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.User), nil
}

type userController struct {
	ns string
	controller.GenericController
}

func (c *userController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *userController) Lister() UserLister {
	return &userLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *userController) AddHandler(ctx context.Context, name string, handler UserHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.User); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler UserHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.User); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler UserHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.User); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler UserHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.User); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type userFactory struct {
}

func (c userFactory) Object() runtime.Object {
	return &v3.User{}
}

func (c userFactory) List() runtime.Object {
	return &v3.UserList{}
}

func (s *userClient) Controller() UserController {
	genericController := controller.NewGenericController(s.ns, UserGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(UserGroupVersionResource, UserGroupVersionKind.Kind, false))

	return &userController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type userClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   UserController
}

func (s *userClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *userClient) Create(o *v3.User) (*v3.User, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.User), err
}

func (s *userClient) Get(name string, opts metav1.GetOptions) (*v3.User, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.User), err
}

func (s *userClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.User, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.User), err
}

func (s *userClient) Update(o *v3.User) (*v3.User, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.User), err
}

func (s *userClient) UpdateStatus(o *v3.User) (*v3.User, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.User), err
}

func (s *userClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *userClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *userClient) List(opts metav1.ListOptions) (*v3.UserList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.UserList), err
}

func (s *userClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.UserList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.UserList), err
}

func (s *userClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *userClient) Patch(o *v3.User, patchType types.PatchType, data []byte, subresources ...string) (*v3.User, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.User), err
}

func (s *userClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *userClient) AddHandler(ctx context.Context, name string, sync UserHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *userClient) AddLifecycle(ctx context.Context, name string, lifecycle UserLifecycle) {
	sync := NewUserLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle UserLifecycle) {
	sync := NewUserLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *userClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync UserHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *userClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserLifecycle) {
	sync := NewUserLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle UserLifecycle) {
	sync := NewUserLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
