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
	RoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RoleBinding",
	}
	RoleBindingResource = metav1.APIResource{
		Name:         "rolebindings",
		SingularName: "rolebinding",
		Namespaced:   true,

		Kind: RoleBindingGroupVersionKind.Kind,
	}

	RoleBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rolebindings",
	}
)

func init() {
	resource.Put(RoleBindingGroupVersionResource)
}

func NewRoleBinding(namespace, name string, obj v1.RoleBinding) *v1.RoleBinding {
	obj.APIVersion, obj.Kind = RoleBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.RoleBinding `json:"items"`
}

type RoleBindingHandlerFunc func(key string, obj *v1.RoleBinding) (runtime.Object, error)

type RoleBindingChangeHandlerFunc func(obj *v1.RoleBinding) (runtime.Object, error)

type RoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.RoleBinding, err error)
	Get(namespace, name string) (*v1.RoleBinding, error)
}

type RoleBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RoleBindingLister
	AddHandler(ctx context.Context, name string, handler RoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RoleBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.RoleBinding) (*v1.RoleBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	Update(*v1.RoleBinding) (*v1.RoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RoleBindingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*RoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleBindingController
	AddHandler(ctx context.Context, name string, sync RoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RoleBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleBindingLifecycle)
}

type roleBindingLister struct {
	controller *roleBindingController
}

func (l *roleBindingLister) List(namespace string, selector labels.Selector) (ret []*v1.RoleBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.RoleBinding))
	})
	return
}

func (l *roleBindingLister) Get(namespace, name string) (*v1.RoleBinding, error) {
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
			Group:    RoleBindingGroupVersionKind.Group,
			Resource: "roleBinding",
		}, key)
	}
	return obj.(*v1.RoleBinding), nil
}

type roleBindingController struct {
	controller.GenericController
}

func (c *roleBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *roleBindingController) Lister() RoleBindingLister {
	return &roleBindingLister{
		controller: c,
	}
}

func (c *roleBindingController) AddHandler(ctx context.Context, name string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type roleBindingFactory struct {
}

func (c roleBindingFactory) Object() runtime.Object {
	return &v1.RoleBinding{}
}

func (c roleBindingFactory) List() runtime.Object {
	return &RoleBindingList{}
}

func (s *roleBindingClient) Controller() RoleBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.roleBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RoleBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &roleBindingController{
		GenericController: genericController,
	}

	s.client.roleBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type roleBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RoleBindingController
}

func (s *roleBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *roleBindingClient) Create(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Get(name string, opts metav1.GetOptions) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Update(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *roleBindingClient) List(opts metav1.ListOptions) (*RoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RoleBindingList), err
}

func (s *roleBindingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*RoleBindingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*RoleBindingList), err
}

func (s *roleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *roleBindingClient) Patch(o *v1.RoleBinding, patchType types.PatchType, data []byte, subresources ...string) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleBindingClient) AddHandler(ctx context.Context, name string, sync RoleBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *roleBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type RoleBindingIndexer func(obj *v1.RoleBinding) ([]string, error)

type RoleBindingClientCache interface {
	Get(namespace, name string) (*v1.RoleBinding, error)
	List(namespace string, selector labels.Selector) ([]*v1.RoleBinding, error)

	Index(name string, indexer RoleBindingIndexer)
	GetIndexed(name, key string) ([]*v1.RoleBinding, error)
}

type RoleBindingClient interface {
	Create(*v1.RoleBinding) (*v1.RoleBinding, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	Update(*v1.RoleBinding) (*v1.RoleBinding, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*RoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() RoleBindingClientCache

	OnCreate(ctx context.Context, name string, sync RoleBindingChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync RoleBindingChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync RoleBindingChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() RoleBindingInterface
}

type roleBindingClientCache struct {
	client *roleBindingClient2
}

type roleBindingClient2 struct {
	iface      RoleBindingInterface
	controller RoleBindingController
}

func (n *roleBindingClient2) Interface() RoleBindingInterface {
	return n.iface
}

func (n *roleBindingClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *roleBindingClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *roleBindingClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *roleBindingClient2) Create(obj *v1.RoleBinding) (*v1.RoleBinding, error) {
	return n.iface.Create(obj)
}

func (n *roleBindingClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.RoleBinding, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *roleBindingClient2) Update(obj *v1.RoleBinding) (*v1.RoleBinding, error) {
	return n.iface.Update(obj)
}

func (n *roleBindingClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *roleBindingClient2) List(namespace string, opts metav1.ListOptions) (*RoleBindingList, error) {
	return n.iface.List(opts)
}

func (n *roleBindingClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *roleBindingClientCache) Get(namespace, name string) (*v1.RoleBinding, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *roleBindingClientCache) List(namespace string, selector labels.Selector) ([]*v1.RoleBinding, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *roleBindingClient2) Cache() RoleBindingClientCache {
	n.loadController()
	return &roleBindingClientCache{
		client: n,
	}
}

func (n *roleBindingClient2) OnCreate(ctx context.Context, name string, sync RoleBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &roleBindingLifecycleDelegate{create: sync})
}

func (n *roleBindingClient2) OnChange(ctx context.Context, name string, sync RoleBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &roleBindingLifecycleDelegate{update: sync})
}

func (n *roleBindingClient2) OnRemove(ctx context.Context, name string, sync RoleBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &roleBindingLifecycleDelegate{remove: sync})
}

func (n *roleBindingClientCache) Index(name string, indexer RoleBindingIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.RoleBinding); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *roleBindingClientCache) GetIndexed(name, key string) ([]*v1.RoleBinding, error) {
	var result []*v1.RoleBinding
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.RoleBinding); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *roleBindingClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type roleBindingLifecycleDelegate struct {
	create RoleBindingChangeHandlerFunc
	update RoleBindingChangeHandlerFunc
	remove RoleBindingChangeHandlerFunc
}

func (n *roleBindingLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *roleBindingLifecycleDelegate) Create(obj *v1.RoleBinding) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *roleBindingLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *roleBindingLifecycleDelegate) Remove(obj *v1.RoleBinding) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *roleBindingLifecycleDelegate) Updated(obj *v1.RoleBinding) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
