package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v1 "k8s.io/api/rbac/v1"
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
	RoleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Role",
	}
	RoleResource = metav1.APIResource{
		Name:         "roles",
		SingularName: "role",
		Namespaced:   true,

		Kind: RoleGroupVersionKind.Kind,
	}

	RoleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "roles",
	}
)

func init() {
	resource.Put(RoleGroupVersionResource)
}

func NewRole(namespace, name string, obj v1.Role) *v1.Role {
	obj.APIVersion, obj.Kind = RoleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Role `json:"items"`
}

type RoleHandlerFunc func(key string, obj *v1.Role) (runtime.Object, error)

type RoleChangeHandlerFunc func(obj *v1.Role) (runtime.Object, error)

type RoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Role, err error)
	Get(namespace, name string) (*v1.Role, error)
}

type RoleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RoleLister
	AddHandler(ctx context.Context, name string, handler RoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RoleHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RoleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Role) (*v1.Role, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Role, error)
	Get(name string, opts metav1.GetOptions) (*v1.Role, error)
	Update(*v1.Role) (*v1.Role, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RoleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*RoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleController
	AddHandler(ctx context.Context, name string, sync RoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RoleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleLifecycle)
}

type roleLister struct {
	controller *roleController
}

func (l *roleLister) List(namespace string, selector labels.Selector) (ret []*v1.Role, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Role))
	})
	return
}

func (l *roleLister) Get(namespace, name string) (*v1.Role, error) {
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
			Group:    RoleGroupVersionKind.Group,
			Resource: "role",
		}, key)
	}
	return obj.(*v1.Role), nil
}

type roleController struct {
	controller.GenericController
}

func (c *roleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *roleController) Lister() RoleLister {
	return &roleLister{
		controller: c,
	}
}

func (c *roleController) AddHandler(ctx context.Context, name string, handler RoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Role); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Role); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Role); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Role); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type roleFactory struct {
}

func (c roleFactory) Object() runtime.Object {
	return &v1.Role{}
}

func (c roleFactory) List() runtime.Object {
	return &RoleList{}
}

func (s *roleClient) Controller() RoleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.roleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RoleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &roleController{
		GenericController: genericController,
	}

	s.client.roleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type roleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RoleController
}

func (s *roleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *roleClient) Create(o *v1.Role) (*v1.Role, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Role), err
}

func (s *roleClient) Get(name string, opts metav1.GetOptions) (*v1.Role, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Role), err
}

func (s *roleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Role, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Role), err
}

func (s *roleClient) Update(o *v1.Role) (*v1.Role, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Role), err
}

func (s *roleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *roleClient) List(opts metav1.ListOptions) (*RoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RoleList), err
}

func (s *roleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*RoleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*RoleList), err
}

func (s *roleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *roleClient) Patch(o *v1.Role, patchType types.PatchType, data []byte, subresources ...string) (*v1.Role, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Role), err
}

func (s *roleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleClient) AddHandler(ctx context.Context, name string, sync RoleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleClient) AddLifecycle(ctx context.Context, name string, lifecycle RoleLifecycle) {
	sync := NewRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleLifecycle) {
	sync := NewRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *roleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleLifecycle) {
	sync := NewRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleLifecycle) {
	sync := NewRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type RoleIndexer func(obj *v1.Role) ([]string, error)

type RoleClientCache interface {
	Get(namespace, name string) (*v1.Role, error)
	List(namespace string, selector labels.Selector) ([]*v1.Role, error)

	Index(name string, indexer RoleIndexer)
	GetIndexed(name, key string) ([]*v1.Role, error)
}

type RoleClient interface {
	Create(*v1.Role) (*v1.Role, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Role, error)
	Update(*v1.Role) (*v1.Role, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*RoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() RoleClientCache

	OnCreate(ctx context.Context, name string, sync RoleChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync RoleChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync RoleChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() RoleInterface
}

type roleClientCache struct {
	client *roleClient2
}

type roleClient2 struct {
	iface      RoleInterface
	controller RoleController
}

func (n *roleClient2) Interface() RoleInterface {
	return n.iface
}

func (n *roleClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *roleClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *roleClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *roleClient2) Create(obj *v1.Role) (*v1.Role, error) {
	return n.iface.Create(obj)
}

func (n *roleClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Role, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *roleClient2) Update(obj *v1.Role) (*v1.Role, error) {
	return n.iface.Update(obj)
}

func (n *roleClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *roleClient2) List(namespace string, opts metav1.ListOptions) (*RoleList, error) {
	return n.iface.List(opts)
}

func (n *roleClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *roleClientCache) Get(namespace, name string) (*v1.Role, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *roleClientCache) List(namespace string, selector labels.Selector) ([]*v1.Role, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *roleClient2) Cache() RoleClientCache {
	n.loadController()
	return &roleClientCache{
		client: n,
	}
}

func (n *roleClient2) OnCreate(ctx context.Context, name string, sync RoleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &roleLifecycleDelegate{create: sync})
}

func (n *roleClient2) OnChange(ctx context.Context, name string, sync RoleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &roleLifecycleDelegate{update: sync})
}

func (n *roleClient2) OnRemove(ctx context.Context, name string, sync RoleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &roleLifecycleDelegate{remove: sync})
}

func (n *roleClientCache) Index(name string, indexer RoleIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Role); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *roleClientCache) GetIndexed(name, key string) ([]*v1.Role, error) {
	var result []*v1.Role
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Role); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *roleClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type roleLifecycleDelegate struct {
	create RoleChangeHandlerFunc
	update RoleChangeHandlerFunc
	remove RoleChangeHandlerFunc
}

func (n *roleLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *roleLifecycleDelegate) Create(obj *v1.Role) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *roleLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *roleLifecycleDelegate) Remove(obj *v1.Role) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *roleLifecycleDelegate) Updated(obj *v1.Role) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
