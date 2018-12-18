package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
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
)

func NewUser(namespace, name string, obj User) *User {
	obj.APIVersion, obj.Kind = UserGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User
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
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler UserHandlerFunc)
	Enqueue(namespace, name string)
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
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() UserController
	AddHandler(ctx context.Context, name string, sync UserHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle UserLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserLifecycle)
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

func (s *userClient) AddLifecycle(ctx context.Context, name string, lifecycle UserLifecycle) {
	sync := NewUserLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserLifecycle) {
	sync := NewUserLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type UserIndexer func(obj *User) ([]string, error)

type UserClientCache interface {
	Get(namespace, name string) (*User, error)
	List(namespace string, selector labels.Selector) ([]*User, error)

	Index(name string, indexer UserIndexer)
	GetIndexed(name, key string) ([]*User, error)
}

type UserClient interface {
	Create(*User) (*User, error)
	Get(namespace, name string, opts metav1.GetOptions) (*User, error)
	Update(*User) (*User, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*UserList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() UserClientCache

	OnCreate(ctx context.Context, name string, sync UserChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync UserChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync UserChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() UserInterface
}

type userClientCache struct {
	client *userClient2
}

type userClient2 struct {
	iface      UserInterface
	controller UserController
}

func (n *userClient2) Interface() UserInterface {
	return n.iface
}

func (n *userClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *userClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *userClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *userClient2) Create(obj *User) (*User, error) {
	return n.iface.Create(obj)
}

func (n *userClient2) Get(namespace, name string, opts metav1.GetOptions) (*User, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *userClient2) Update(obj *User) (*User, error) {
	return n.iface.Update(obj)
}

func (n *userClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *userClient2) List(namespace string, opts metav1.ListOptions) (*UserList, error) {
	return n.iface.List(opts)
}

func (n *userClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *userClientCache) Get(namespace, name string) (*User, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *userClientCache) List(namespace string, selector labels.Selector) ([]*User, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *userClient2) Cache() UserClientCache {
	n.loadController()
	return &userClientCache{
		client: n,
	}
}

func (n *userClient2) OnCreate(ctx context.Context, name string, sync UserChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &userLifecycleDelegate{create: sync})
}

func (n *userClient2) OnChange(ctx context.Context, name string, sync UserChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &userLifecycleDelegate{update: sync})
}

func (n *userClient2) OnRemove(ctx context.Context, name string, sync UserChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &userLifecycleDelegate{remove: sync})
}

func (n *userClientCache) Index(name string, indexer UserIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*User); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *userClientCache) GetIndexed(name, key string) ([]*User, error) {
	var result []*User
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*User); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *userClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type userLifecycleDelegate struct {
	create UserChangeHandlerFunc
	update UserChangeHandlerFunc
	remove UserChangeHandlerFunc
}

func (n *userLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *userLifecycleDelegate) Create(obj *User) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *userLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *userLifecycleDelegate) Remove(obj *User) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *userLifecycleDelegate) Updated(obj *User) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
