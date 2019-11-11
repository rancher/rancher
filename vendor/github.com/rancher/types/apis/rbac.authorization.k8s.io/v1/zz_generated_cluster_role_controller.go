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
	ClusterRoleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRole",
	}
	ClusterRoleResource = metav1.APIResource{
		Name:         "clusterroles",
		SingularName: "clusterrole",
		Namespaced:   false,
		Kind:         ClusterRoleGroupVersionKind.Kind,
	}

	ClusterRoleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterroles",
	}
)

func init() {
	resource.Put(ClusterRoleGroupVersionResource)
}

func NewClusterRole(namespace, name string, obj v1.ClusterRole) *v1.ClusterRole {
	obj.APIVersion, obj.Kind = ClusterRoleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ClusterRole `json:"items"`
}

type ClusterRoleHandlerFunc func(key string, obj *v1.ClusterRole) (runtime.Object, error)

type ClusterRoleChangeHandlerFunc func(obj *v1.ClusterRole) (runtime.Object, error)

type ClusterRoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ClusterRole, err error)
	Get(namespace, name string) (*v1.ClusterRole, error)
}

type ClusterRoleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleLister
	AddHandler(ctx context.Context, name string, handler ClusterRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterRoleHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRoleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ClusterRole) (*v1.ClusterRole, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	Get(name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	Update(*v1.ClusterRole) (*v1.ClusterRole, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRoleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterRoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleController
	AddHandler(ctx context.Context, name string, sync ClusterRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleLifecycle)
}

type clusterRoleLister struct {
	controller *clusterRoleController
}

func (l *clusterRoleLister) List(namespace string, selector labels.Selector) (ret []*v1.ClusterRole, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ClusterRole))
	})
	return
}

func (l *clusterRoleLister) Get(namespace, name string) (*v1.ClusterRole, error) {
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
			Group:    ClusterRoleGroupVersionKind.Group,
			Resource: "clusterRole",
		}, key)
	}
	return obj.(*v1.ClusterRole), nil
}

type clusterRoleController struct {
	controller.GenericController
}

func (c *clusterRoleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRoleController) Lister() ClusterRoleLister {
	return &clusterRoleLister{
		controller: c,
	}
}

func (c *clusterRoleController) AddHandler(ctx context.Context, name string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRoleFactory struct {
}

func (c clusterRoleFactory) Object() runtime.Object {
	return &v1.ClusterRole{}
}

func (c clusterRoleFactory) List() runtime.Object {
	return &ClusterRoleList{}
}

func (s *clusterRoleClient) Controller() ClusterRoleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRoleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRoleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRoleController{
		GenericController: genericController,
	}

	s.client.clusterRoleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRoleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRoleController
}

func (s *clusterRoleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleClient) Create(o *v1.ClusterRole) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Get(name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Update(o *v1.ClusterRole) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRoleClient) List(opts metav1.ListOptions) (*ClusterRoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRoleList), err
}

func (s *clusterRoleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterRoleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterRoleList), err
}

func (s *clusterRoleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleClient) Patch(o *v1.ClusterRole, patchType types.PatchType, data []byte, subresources ...string) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleClient) AddHandler(ctx context.Context, name string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterRoleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ClusterRoleIndexer func(obj *v1.ClusterRole) ([]string, error)

type ClusterRoleClientCache interface {
	Get(namespace, name string) (*v1.ClusterRole, error)
	List(namespace string, selector labels.Selector) ([]*v1.ClusterRole, error)

	Index(name string, indexer ClusterRoleIndexer)
	GetIndexed(name, key string) ([]*v1.ClusterRole, error)
}

type ClusterRoleClient interface {
	Create(*v1.ClusterRole) (*v1.ClusterRole, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	Update(*v1.ClusterRole) (*v1.ClusterRole, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterRoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterRoleClientCache

	OnCreate(ctx context.Context, name string, sync ClusterRoleChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterRoleChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterRoleChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterRoleInterface
}

type clusterRoleClientCache struct {
	client *clusterRoleClient2
}

type clusterRoleClient2 struct {
	iface      ClusterRoleInterface
	controller ClusterRoleController
}

func (n *clusterRoleClient2) Interface() ClusterRoleInterface {
	return n.iface
}

func (n *clusterRoleClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterRoleClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterRoleClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterRoleClient2) Create(obj *v1.ClusterRole) (*v1.ClusterRole, error) {
	return n.iface.Create(obj)
}

func (n *clusterRoleClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterRoleClient2) Update(obj *v1.ClusterRole) (*v1.ClusterRole, error) {
	return n.iface.Update(obj)
}

func (n *clusterRoleClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterRoleClient2) List(namespace string, opts metav1.ListOptions) (*ClusterRoleList, error) {
	return n.iface.List(opts)
}

func (n *clusterRoleClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterRoleClientCache) Get(namespace, name string) (*v1.ClusterRole, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterRoleClientCache) List(namespace string, selector labels.Selector) ([]*v1.ClusterRole, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterRoleClient2) Cache() ClusterRoleClientCache {
	n.loadController()
	return &clusterRoleClientCache{
		client: n,
	}
}

func (n *clusterRoleClient2) OnCreate(ctx context.Context, name string, sync ClusterRoleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterRoleLifecycleDelegate{create: sync})
}

func (n *clusterRoleClient2) OnChange(ctx context.Context, name string, sync ClusterRoleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterRoleLifecycleDelegate{update: sync})
}

func (n *clusterRoleClient2) OnRemove(ctx context.Context, name string, sync ClusterRoleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterRoleLifecycleDelegate{remove: sync})
}

func (n *clusterRoleClientCache) Index(name string, indexer ClusterRoleIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.ClusterRole); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterRoleClientCache) GetIndexed(name, key string) ([]*v1.ClusterRole, error) {
	var result []*v1.ClusterRole
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.ClusterRole); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterRoleClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterRoleLifecycleDelegate struct {
	create ClusterRoleChangeHandlerFunc
	update ClusterRoleChangeHandlerFunc
	remove ClusterRoleChangeHandlerFunc
}

func (n *clusterRoleLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterRoleLifecycleDelegate) Create(obj *v1.ClusterRole) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterRoleLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterRoleLifecycleDelegate) Remove(obj *v1.ClusterRole) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterRoleLifecycleDelegate) Updated(obj *v1.ClusterRole) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
