package v3

import (
	"context"

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
	NamespacedSSHAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedSSHAuth",
	}
	NamespacedSSHAuthResource = metav1.APIResource{
		Name:         "namespacedsshauths",
		SingularName: "namespacedsshauth",
		Namespaced:   true,

		Kind: NamespacedSSHAuthGroupVersionKind.Kind,
	}

	NamespacedSSHAuthGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespacedsshauths",
	}
)

func init() {
	resource.Put(NamespacedSSHAuthGroupVersionResource)
}

func NewNamespacedSSHAuth(namespace, name string, obj NamespacedSSHAuth) *NamespacedSSHAuth {
	obj.APIVersion, obj.Kind = NamespacedSSHAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedSSHAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedSSHAuth `json:"items"`
}

type NamespacedSSHAuthHandlerFunc func(key string, obj *NamespacedSSHAuth) (runtime.Object, error)

type NamespacedSSHAuthChangeHandlerFunc func(obj *NamespacedSSHAuth) (runtime.Object, error)

type NamespacedSSHAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedSSHAuth, err error)
	Get(namespace, name string) (*NamespacedSSHAuth, error)
}

type NamespacedSSHAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedSSHAuthLister
	AddHandler(ctx context.Context, name string, handler NamespacedSSHAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedSSHAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedSSHAuthHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedSSHAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error)
	Update(*NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedSSHAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*NamespacedSSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedSSHAuthController
	AddHandler(ctx context.Context, name string, sync NamespacedSSHAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedSSHAuthHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedSSHAuthLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedSSHAuthLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedSSHAuthHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle)
}

type namespacedSshAuthLister struct {
	controller *namespacedSshAuthController
}

func (l *namespacedSshAuthLister) List(namespace string, selector labels.Selector) (ret []*NamespacedSSHAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedSSHAuth))
	})
	return
}

func (l *namespacedSshAuthLister) Get(namespace, name string) (*NamespacedSSHAuth, error) {
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
			Group:    NamespacedSSHAuthGroupVersionKind.Group,
			Resource: "namespacedSshAuth",
		}, key)
	}
	return obj.(*NamespacedSSHAuth), nil
}

type namespacedSshAuthController struct {
	controller.GenericController
}

func (c *namespacedSshAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedSshAuthController) Lister() NamespacedSSHAuthLister {
	return &namespacedSshAuthLister{
		controller: c,
	}
}

func (c *namespacedSshAuthController) AddHandler(ctx context.Context, name string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedSSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedSshAuthController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedSSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedSshAuthController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedSSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedSshAuthController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedSSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedSSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedSshAuthFactory struct {
}

func (c namespacedSshAuthFactory) Object() runtime.Object {
	return &NamespacedSSHAuth{}
}

func (c namespacedSshAuthFactory) List() runtime.Object {
	return &NamespacedSSHAuthList{}
}

func (s *namespacedSshAuthClient) Controller() NamespacedSSHAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedSshAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedSSHAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedSshAuthController{
		GenericController: genericController,
	}

	s.client.namespacedSshAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedSshAuthClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedSSHAuthController
}

func (s *namespacedSshAuthClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedSshAuthClient) Create(o *NamespacedSSHAuth) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Get(name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Update(o *NamespacedSSHAuth) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedSshAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedSshAuthClient) List(opts metav1.ListOptions) (*NamespacedSSHAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedSSHAuthList), err
}

func (s *namespacedSshAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*NamespacedSSHAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*NamespacedSSHAuthList), err
}

func (s *namespacedSshAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedSshAuthClient) Patch(o *NamespacedSSHAuth, patchType types.PatchType, data []byte, subresources ...string) (*NamespacedSSHAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*NamespacedSSHAuth), err
}

func (s *namespacedSshAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedSshAuthClient) AddHandler(ctx context.Context, name string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedSshAuthClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedSshAuthClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedSshAuthClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedSSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedSshAuthClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedSSHAuthLifecycle) {
	sync := NewNamespacedSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type NamespacedSSHAuthIndexer func(obj *NamespacedSSHAuth) ([]string, error)

type NamespacedSSHAuthClientCache interface {
	Get(namespace, name string) (*NamespacedSSHAuth, error)
	List(namespace string, selector labels.Selector) ([]*NamespacedSSHAuth, error)

	Index(name string, indexer NamespacedSSHAuthIndexer)
	GetIndexed(name, key string) ([]*NamespacedSSHAuth, error)
}

type NamespacedSSHAuthClient interface {
	Create(*NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	Get(namespace, name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error)
	Update(*NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*NamespacedSSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() NamespacedSSHAuthClientCache

	OnCreate(ctx context.Context, name string, sync NamespacedSSHAuthChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync NamespacedSSHAuthChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync NamespacedSSHAuthChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() NamespacedSSHAuthInterface
}

type namespacedSshAuthClientCache struct {
	client *namespacedSshAuthClient2
}

type namespacedSshAuthClient2 struct {
	iface      NamespacedSSHAuthInterface
	controller NamespacedSSHAuthController
}

func (n *namespacedSshAuthClient2) Interface() NamespacedSSHAuthInterface {
	return n.iface
}

func (n *namespacedSshAuthClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *namespacedSshAuthClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *namespacedSshAuthClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *namespacedSshAuthClient2) Create(obj *NamespacedSSHAuth) (*NamespacedSSHAuth, error) {
	return n.iface.Create(obj)
}

func (n *namespacedSshAuthClient2) Get(namespace, name string, opts metav1.GetOptions) (*NamespacedSSHAuth, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *namespacedSshAuthClient2) Update(obj *NamespacedSSHAuth) (*NamespacedSSHAuth, error) {
	return n.iface.Update(obj)
}

func (n *namespacedSshAuthClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *namespacedSshAuthClient2) List(namespace string, opts metav1.ListOptions) (*NamespacedSSHAuthList, error) {
	return n.iface.List(opts)
}

func (n *namespacedSshAuthClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *namespacedSshAuthClientCache) Get(namespace, name string) (*NamespacedSSHAuth, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *namespacedSshAuthClientCache) List(namespace string, selector labels.Selector) ([]*NamespacedSSHAuth, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *namespacedSshAuthClient2) Cache() NamespacedSSHAuthClientCache {
	n.loadController()
	return &namespacedSshAuthClientCache{
		client: n,
	}
}

func (n *namespacedSshAuthClient2) OnCreate(ctx context.Context, name string, sync NamespacedSSHAuthChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &namespacedSshAuthLifecycleDelegate{create: sync})
}

func (n *namespacedSshAuthClient2) OnChange(ctx context.Context, name string, sync NamespacedSSHAuthChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &namespacedSshAuthLifecycleDelegate{update: sync})
}

func (n *namespacedSshAuthClient2) OnRemove(ctx context.Context, name string, sync NamespacedSSHAuthChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &namespacedSshAuthLifecycleDelegate{remove: sync})
}

func (n *namespacedSshAuthClientCache) Index(name string, indexer NamespacedSSHAuthIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*NamespacedSSHAuth); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *namespacedSshAuthClientCache) GetIndexed(name, key string) ([]*NamespacedSSHAuth, error) {
	var result []*NamespacedSSHAuth
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*NamespacedSSHAuth); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *namespacedSshAuthClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type namespacedSshAuthLifecycleDelegate struct {
	create NamespacedSSHAuthChangeHandlerFunc
	update NamespacedSSHAuthChangeHandlerFunc
	remove NamespacedSSHAuthChangeHandlerFunc
}

func (n *namespacedSshAuthLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *namespacedSshAuthLifecycleDelegate) Create(obj *NamespacedSSHAuth) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *namespacedSshAuthLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *namespacedSshAuthLifecycleDelegate) Remove(obj *NamespacedSSHAuth) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *namespacedSshAuthLifecycleDelegate) Updated(obj *NamespacedSSHAuth) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
