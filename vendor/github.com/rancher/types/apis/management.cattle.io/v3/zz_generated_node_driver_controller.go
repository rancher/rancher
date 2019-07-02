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
	NodeDriverGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NodeDriver",
	}
	NodeDriverResource = metav1.APIResource{
		Name:         "nodedrivers",
		SingularName: "nodedriver",
		Namespaced:   false,
		Kind:         NodeDriverGroupVersionKind.Kind,
	}

	NodeDriverGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "nodedrivers",
	}
)

func init() {
	resource.Put(NodeDriverGroupVersionResource)
}

func NewNodeDriver(namespace, name string, obj NodeDriver) *NodeDriver {
	obj.APIVersion, obj.Kind = NodeDriverGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodeDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeDriver `json:"items"`
}

type NodeDriverHandlerFunc func(key string, obj *NodeDriver) (runtime.Object, error)

type NodeDriverChangeHandlerFunc func(obj *NodeDriver) (runtime.Object, error)

type NodeDriverLister interface {
	List(namespace string, selector labels.Selector) (ret []*NodeDriver, err error)
	Get(namespace, name string) (*NodeDriver, error)
}

type NodeDriverController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NodeDriverLister
	AddHandler(ctx context.Context, name string, handler NodeDriverHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeDriverHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NodeDriverHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NodeDriverHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NodeDriverInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NodeDriver) (*NodeDriver, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodeDriver, error)
	Get(name string, opts metav1.GetOptions) (*NodeDriver, error)
	Update(*NodeDriver) (*NodeDriver, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NodeDriverList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeDriverController
	AddHandler(ctx context.Context, name string, sync NodeDriverHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeDriverHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NodeDriverLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeDriverLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeDriverHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeDriverHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeDriverLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeDriverLifecycle)
}

type nodeDriverLister struct {
	controller *nodeDriverController
}

func (l *nodeDriverLister) List(namespace string, selector labels.Selector) (ret []*NodeDriver, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NodeDriver))
	})
	return
}

func (l *nodeDriverLister) Get(namespace, name string) (*NodeDriver, error) {
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
			Group:    NodeDriverGroupVersionKind.Group,
			Resource: "nodeDriver",
		}, key)
	}
	return obj.(*NodeDriver), nil
}

type nodeDriverController struct {
	controller.GenericController
}

func (c *nodeDriverController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodeDriverController) Lister() NodeDriverLister {
	return &nodeDriverLister{
		controller: c,
	}
}

func (c *nodeDriverController) AddHandler(ctx context.Context, name string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeDriver); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeDriverController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeDriver); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeDriverController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NodeDriverHandlerFunc) {
	resource.PutClusterScoped(NodeDriverGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeDriver); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeDriverController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NodeDriverHandlerFunc) {
	resource.PutClusterScoped(NodeDriverGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeDriver); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodeDriverFactory struct {
}

func (c nodeDriverFactory) Object() runtime.Object {
	return &NodeDriver{}
}

func (c nodeDriverFactory) List() runtime.Object {
	return &NodeDriverList{}
}

func (s *nodeDriverClient) Controller() NodeDriverController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.nodeDriverControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NodeDriverGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &nodeDriverController{
		GenericController: genericController,
	}

	s.client.nodeDriverControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type nodeDriverClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NodeDriverController
}

func (s *nodeDriverClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *nodeDriverClient) Create(o *NodeDriver) (*NodeDriver, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) Get(name string, opts metav1.GetOptions) (*NodeDriver, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodeDriver, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) Update(o *NodeDriver) (*NodeDriver, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeDriverClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeDriverClient) List(opts metav1.ListOptions) (*NodeDriverList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NodeDriverList), err
}

func (s *nodeDriverClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeDriverClient) Patch(o *NodeDriver, patchType types.PatchType, data []byte, subresources ...string) (*NodeDriver, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeDriverClient) AddHandler(ctx context.Context, name string, sync NodeDriverHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeDriverClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeDriverHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeDriverClient) AddLifecycle(ctx context.Context, name string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeDriverClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeDriverClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeDriverHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeDriverHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type NodeDriverIndexer func(obj *NodeDriver) ([]string, error)

type NodeDriverClientCache interface {
	Get(namespace, name string) (*NodeDriver, error)
	List(namespace string, selector labels.Selector) ([]*NodeDriver, error)

	Index(name string, indexer NodeDriverIndexer)
	GetIndexed(name, key string) ([]*NodeDriver, error)
}

type NodeDriverClient interface {
	Create(*NodeDriver) (*NodeDriver, error)
	Get(namespace, name string, opts metav1.GetOptions) (*NodeDriver, error)
	Update(*NodeDriver) (*NodeDriver, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*NodeDriverList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() NodeDriverClientCache

	OnCreate(ctx context.Context, name string, sync NodeDriverChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync NodeDriverChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync NodeDriverChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() NodeDriverInterface
}

type nodeDriverClientCache struct {
	client *nodeDriverClient2
}

type nodeDriverClient2 struct {
	iface      NodeDriverInterface
	controller NodeDriverController
}

func (n *nodeDriverClient2) Interface() NodeDriverInterface {
	return n.iface
}

func (n *nodeDriverClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *nodeDriverClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *nodeDriverClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *nodeDriverClient2) Create(obj *NodeDriver) (*NodeDriver, error) {
	return n.iface.Create(obj)
}

func (n *nodeDriverClient2) Get(namespace, name string, opts metav1.GetOptions) (*NodeDriver, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *nodeDriverClient2) Update(obj *NodeDriver) (*NodeDriver, error) {
	return n.iface.Update(obj)
}

func (n *nodeDriverClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *nodeDriverClient2) List(namespace string, opts metav1.ListOptions) (*NodeDriverList, error) {
	return n.iface.List(opts)
}

func (n *nodeDriverClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *nodeDriverClientCache) Get(namespace, name string) (*NodeDriver, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *nodeDriverClientCache) List(namespace string, selector labels.Selector) ([]*NodeDriver, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *nodeDriverClient2) Cache() NodeDriverClientCache {
	n.loadController()
	return &nodeDriverClientCache{
		client: n,
	}
}

func (n *nodeDriverClient2) OnCreate(ctx context.Context, name string, sync NodeDriverChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &nodeDriverLifecycleDelegate{create: sync})
}

func (n *nodeDriverClient2) OnChange(ctx context.Context, name string, sync NodeDriverChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &nodeDriverLifecycleDelegate{update: sync})
}

func (n *nodeDriverClient2) OnRemove(ctx context.Context, name string, sync NodeDriverChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &nodeDriverLifecycleDelegate{remove: sync})
}

func (n *nodeDriverClientCache) Index(name string, indexer NodeDriverIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*NodeDriver); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *nodeDriverClientCache) GetIndexed(name, key string) ([]*NodeDriver, error) {
	var result []*NodeDriver
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*NodeDriver); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *nodeDriverClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type nodeDriverLifecycleDelegate struct {
	create NodeDriverChangeHandlerFunc
	update NodeDriverChangeHandlerFunc
	remove NodeDriverChangeHandlerFunc
}

func (n *nodeDriverLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *nodeDriverLifecycleDelegate) Create(obj *NodeDriver) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *nodeDriverLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *nodeDriverLifecycleDelegate) Remove(obj *NodeDriver) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *nodeDriverLifecycleDelegate) Updated(obj *NodeDriver) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
