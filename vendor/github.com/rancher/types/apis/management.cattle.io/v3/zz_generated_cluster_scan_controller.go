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
	ClusterScanGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterScan",
	}
	ClusterScanResource = metav1.APIResource{
		Name:         "clusterscans",
		SingularName: "clusterscan",
		Namespaced:   true,

		Kind: ClusterScanGroupVersionKind.Kind,
	}

	ClusterScanGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterscans",
	}
)

func init() {
	resource.Put(ClusterScanGroupVersionResource)
}

func NewClusterScan(namespace, name string, obj ClusterScan) *ClusterScan {
	obj.APIVersion, obj.Kind = ClusterScanGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

type ClusterScanHandlerFunc func(key string, obj *ClusterScan) (runtime.Object, error)

type ClusterScanChangeHandlerFunc func(obj *ClusterScan) (runtime.Object, error)

type ClusterScanLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterScan, err error)
	Get(namespace, name string) (*ClusterScan, error)
}

type ClusterScanController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterScanLister
	AddHandler(ctx context.Context, name string, handler ClusterScanHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterScanHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterScanHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterScanHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterScanInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterScan) (*ClusterScan, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterScan, error)
	Get(name string, opts metav1.GetOptions) (*ClusterScan, error)
	Update(*ClusterScan) (*ClusterScan, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterScanList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterScanController
	AddHandler(ctx context.Context, name string, sync ClusterScanHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterScanHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterScanLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterScanLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterScanHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterScanHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterScanLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterScanLifecycle)
}

type clusterScanLister struct {
	controller *clusterScanController
}

func (l *clusterScanLister) List(namespace string, selector labels.Selector) (ret []*ClusterScan, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterScan))
	})
	return
}

func (l *clusterScanLister) Get(namespace, name string) (*ClusterScan, error) {
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
			Group:    ClusterScanGroupVersionKind.Group,
			Resource: "clusterScan",
		}, key)
	}
	return obj.(*ClusterScan), nil
}

type clusterScanController struct {
	controller.GenericController
}

func (c *clusterScanController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterScanController) Lister() ClusterScanLister {
	return &clusterScanLister{
		controller: c,
	}
}

func (c *clusterScanController) AddHandler(ctx context.Context, name string, handler ClusterScanHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterScan); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterScanController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterScanHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterScan); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterScanController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterScanHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterScan); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterScanController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterScanHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterScan); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterScanFactory struct {
}

func (c clusterScanFactory) Object() runtime.Object {
	return &ClusterScan{}
}

func (c clusterScanFactory) List() runtime.Object {
	return &ClusterScanList{}
}

func (s *clusterScanClient) Controller() ClusterScanController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterScanControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterScanGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterScanController{
		GenericController: genericController,
	}

	s.client.clusterScanControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterScanClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterScanController
}

func (s *clusterScanClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterScanClient) Create(o *ClusterScan) (*ClusterScan, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterScan), err
}

func (s *clusterScanClient) Get(name string, opts metav1.GetOptions) (*ClusterScan, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterScan), err
}

func (s *clusterScanClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterScan, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterScan), err
}

func (s *clusterScanClient) Update(o *ClusterScan) (*ClusterScan, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterScan), err
}

func (s *clusterScanClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterScanClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterScanClient) List(opts metav1.ListOptions) (*ClusterScanList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterScanList), err
}

func (s *clusterScanClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterScanClient) Patch(o *ClusterScan, patchType types.PatchType, data []byte, subresources ...string) (*ClusterScan, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterScan), err
}

func (s *clusterScanClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterScanClient) AddHandler(ctx context.Context, name string, sync ClusterScanHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterScanClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterScanHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterScanClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterScanLifecycle) {
	sync := NewClusterScanLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterScanClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterScanLifecycle) {
	sync := NewClusterScanLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterScanClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterScanHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterScanClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterScanHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterScanClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterScanLifecycle) {
	sync := NewClusterScanLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterScanClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterScanLifecycle) {
	sync := NewClusterScanLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ClusterScanIndexer func(obj *ClusterScan) ([]string, error)

type ClusterScanClientCache interface {
	Get(namespace, name string) (*ClusterScan, error)
	List(namespace string, selector labels.Selector) ([]*ClusterScan, error)

	Index(name string, indexer ClusterScanIndexer)
	GetIndexed(name, key string) ([]*ClusterScan, error)
}

type ClusterScanClient interface {
	Create(*ClusterScan) (*ClusterScan, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ClusterScan, error)
	Update(*ClusterScan) (*ClusterScan, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterScanList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterScanClientCache

	OnCreate(ctx context.Context, name string, sync ClusterScanChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterScanChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterScanChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterScanInterface
}

type clusterScanClientCache struct {
	client *clusterScanClient2
}

type clusterScanClient2 struct {
	iface      ClusterScanInterface
	controller ClusterScanController
}

func (n *clusterScanClient2) Interface() ClusterScanInterface {
	return n.iface
}

func (n *clusterScanClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterScanClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterScanClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterScanClient2) Create(obj *ClusterScan) (*ClusterScan, error) {
	return n.iface.Create(obj)
}

func (n *clusterScanClient2) Get(namespace, name string, opts metav1.GetOptions) (*ClusterScan, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterScanClient2) Update(obj *ClusterScan) (*ClusterScan, error) {
	return n.iface.Update(obj)
}

func (n *clusterScanClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterScanClient2) List(namespace string, opts metav1.ListOptions) (*ClusterScanList, error) {
	return n.iface.List(opts)
}

func (n *clusterScanClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterScanClientCache) Get(namespace, name string) (*ClusterScan, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterScanClientCache) List(namespace string, selector labels.Selector) ([]*ClusterScan, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterScanClient2) Cache() ClusterScanClientCache {
	n.loadController()
	return &clusterScanClientCache{
		client: n,
	}
}

func (n *clusterScanClient2) OnCreate(ctx context.Context, name string, sync ClusterScanChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterScanLifecycleDelegate{create: sync})
}

func (n *clusterScanClient2) OnChange(ctx context.Context, name string, sync ClusterScanChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterScanLifecycleDelegate{update: sync})
}

func (n *clusterScanClient2) OnRemove(ctx context.Context, name string, sync ClusterScanChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterScanLifecycleDelegate{remove: sync})
}

func (n *clusterScanClientCache) Index(name string, indexer ClusterScanIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ClusterScan); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterScanClientCache) GetIndexed(name, key string) ([]*ClusterScan, error) {
	var result []*ClusterScan
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ClusterScan); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterScanClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterScanLifecycleDelegate struct {
	create ClusterScanChangeHandlerFunc
	update ClusterScanChangeHandlerFunc
	remove ClusterScanChangeHandlerFunc
}

func (n *clusterScanLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterScanLifecycleDelegate) Create(obj *ClusterScan) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterScanLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterScanLifecycleDelegate) Remove(obj *ClusterScan) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterScanLifecycleDelegate) Updated(obj *ClusterScan) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
