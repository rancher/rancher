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

func NewUser(namespace, name string, obj User) *User {
	obj.APIVersion, obj.Kind = UserGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

type UserHandlerFunc func(key string, obj *User) (runtime.Object, error)

type UserChangeHandlerFunc func(obj *User) (runtime.Object, error)

type UserLister interface {
	List(namespace string, selector labels.Selector) (ret []*User, err error)
	Get(namespace, name string) (*User, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type UserInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*User) (*User, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*User, error)
	Get(name string, opts metav1.GetOptions) (*User, error)
	Update(*User) (*User, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*UserList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*UserList, error)
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
	controller *userController
}

func (l *userLister) List(namespace string, selector labels.Selector) (ret []*User, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*User))
	})
	return
}

func (l *userLister) Get(namespace, name string) (*User, error) {
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
			Resource: "user",
		}, key)
	}
	return obj.(*User), nil
}

type userController struct {
	controller.GenericController
}

func (c *userController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *userController) Lister() UserLister {
	return &userLister{
		controller: c,
	}
}

func (c *userController) AddHandler(ctx context.Context, name string, handler UserHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*User); ok {
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
		} else if v, ok := obj.(*User); ok {
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
		} else if v, ok := obj.(*User); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*User); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type userFactory struct {
}

func (c userFactory) Object() runtime.Object {
	return &User{}
}

func (c userFactory) List() runtime.Object {
	return &UserList{}
}

func (s *userClient) Controller() UserController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.userControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(UserGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &userController{
		GenericController: genericController,
	}

	s.client.userControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *userClient) Create(o *User) (*User, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*User), err
}

func (s *userClient) Get(name string, opts metav1.GetOptions) (*User, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*User), err
}

func (s *userClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*User, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*User), err
}

func (s *userClient) Update(o *User) (*User, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*User), err
}

func (s *userClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *userClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *userClient) List(opts metav1.ListOptions) (*UserList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*UserList), err
}

func (s *userClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*UserList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*UserList), err
}

func (s *userClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *userClient) Patch(o *User, patchType types.PatchType, data []byte, subresources ...string) (*User, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*User), err
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
