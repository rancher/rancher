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
	ListenConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ListenConfig",
	}
	ListenConfigResource = metav1.APIResource{
		Name:         "listenconfigs",
		SingularName: "listenconfig",
		Namespaced:   false,
		Kind:         ListenConfigGroupVersionKind.Kind,
	}

	ListenConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "listenconfigs",
	}
)

func init() {
	resource.Put(ListenConfigGroupVersionResource)
}

func NewListenConfig(namespace, name string, obj ListenConfig) *ListenConfig {
	obj.APIVersion, obj.Kind = ListenConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ListenConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ListenConfig `json:"items"`
}

type ListenConfigHandlerFunc func(key string, obj *ListenConfig) (runtime.Object, error)

type ListenConfigChangeHandlerFunc func(obj *ListenConfig) (runtime.Object, error)

type ListenConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*ListenConfig, err error)
	Get(namespace, name string) (*ListenConfig, error)
}

type ListenConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ListenConfigLister
	AddHandler(ctx context.Context, name string, handler ListenConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ListenConfigHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ListenConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ListenConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ListenConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ListenConfig) (*ListenConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ListenConfig, error)
	Get(name string, opts metav1.GetOptions) (*ListenConfig, error)
	Update(*ListenConfig) (*ListenConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ListenConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ListenConfigController
	AddHandler(ctx context.Context, name string, sync ListenConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ListenConfigHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ListenConfigLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ListenConfigLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ListenConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ListenConfigHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ListenConfigLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ListenConfigLifecycle)
}

type listenConfigLister struct {
	controller *listenConfigController
}

func (l *listenConfigLister) List(namespace string, selector labels.Selector) (ret []*ListenConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ListenConfig))
	})
	return
}

func (l *listenConfigLister) Get(namespace, name string) (*ListenConfig, error) {
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
			Group:    ListenConfigGroupVersionKind.Group,
			Resource: "listenConfig",
		}, key)
	}
	return obj.(*ListenConfig), nil
}

type listenConfigController struct {
	controller.GenericController
}

func (c *listenConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *listenConfigController) Lister() ListenConfigLister {
	return &listenConfigLister{
		controller: c,
	}
}

func (c *listenConfigController) AddHandler(ctx context.Context, name string, handler ListenConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ListenConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *listenConfigController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ListenConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ListenConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *listenConfigController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ListenConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ListenConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *listenConfigController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ListenConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ListenConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type listenConfigFactory struct {
}

func (c listenConfigFactory) Object() runtime.Object {
	return &ListenConfig{}
}

func (c listenConfigFactory) List() runtime.Object {
	return &ListenConfigList{}
}

func (s *listenConfigClient) Controller() ListenConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.listenConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ListenConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &listenConfigController{
		GenericController: genericController,
	}

	s.client.listenConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type listenConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ListenConfigController
}

func (s *listenConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *listenConfigClient) Create(o *ListenConfig) (*ListenConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) Get(name string, opts metav1.GetOptions) (*ListenConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ListenConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) Update(o *ListenConfig) (*ListenConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *listenConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *listenConfigClient) List(opts metav1.ListOptions) (*ListenConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ListenConfigList), err
}

func (s *listenConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *listenConfigClient) Patch(o *ListenConfig, patchType types.PatchType, data []byte, subresources ...string) (*ListenConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ListenConfig), err
}

func (s *listenConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *listenConfigClient) AddHandler(ctx context.Context, name string, sync ListenConfigHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *listenConfigClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ListenConfigHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *listenConfigClient) AddLifecycle(ctx context.Context, name string, lifecycle ListenConfigLifecycle) {
	sync := NewListenConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *listenConfigClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ListenConfigLifecycle) {
	sync := NewListenConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *listenConfigClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ListenConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *listenConfigClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ListenConfigHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *listenConfigClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ListenConfigLifecycle) {
	sync := NewListenConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *listenConfigClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ListenConfigLifecycle) {
	sync := NewListenConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ListenConfigIndexer func(obj *ListenConfig) ([]string, error)

type ListenConfigClientCache interface {
	Get(namespace, name string) (*ListenConfig, error)
	List(namespace string, selector labels.Selector) ([]*ListenConfig, error)

	Index(name string, indexer ListenConfigIndexer)
	GetIndexed(name, key string) ([]*ListenConfig, error)
}

type ListenConfigClient interface {
	Create(*ListenConfig) (*ListenConfig, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ListenConfig, error)
	Update(*ListenConfig) (*ListenConfig, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ListenConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ListenConfigClientCache

	OnCreate(ctx context.Context, name string, sync ListenConfigChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ListenConfigChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ListenConfigChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ListenConfigInterface
}

type listenConfigClientCache struct {
	client *listenConfigClient2
}

type listenConfigClient2 struct {
	iface      ListenConfigInterface
	controller ListenConfigController
}

func (n *listenConfigClient2) Interface() ListenConfigInterface {
	return n.iface
}

func (n *listenConfigClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *listenConfigClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *listenConfigClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *listenConfigClient2) Create(obj *ListenConfig) (*ListenConfig, error) {
	return n.iface.Create(obj)
}

func (n *listenConfigClient2) Get(namespace, name string, opts metav1.GetOptions) (*ListenConfig, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *listenConfigClient2) Update(obj *ListenConfig) (*ListenConfig, error) {
	return n.iface.Update(obj)
}

func (n *listenConfigClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *listenConfigClient2) List(namespace string, opts metav1.ListOptions) (*ListenConfigList, error) {
	return n.iface.List(opts)
}

func (n *listenConfigClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *listenConfigClientCache) Get(namespace, name string) (*ListenConfig, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *listenConfigClientCache) List(namespace string, selector labels.Selector) ([]*ListenConfig, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *listenConfigClient2) Cache() ListenConfigClientCache {
	n.loadController()
	return &listenConfigClientCache{
		client: n,
	}
}

func (n *listenConfigClient2) OnCreate(ctx context.Context, name string, sync ListenConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &listenConfigLifecycleDelegate{create: sync})
}

func (n *listenConfigClient2) OnChange(ctx context.Context, name string, sync ListenConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &listenConfigLifecycleDelegate{update: sync})
}

func (n *listenConfigClient2) OnRemove(ctx context.Context, name string, sync ListenConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &listenConfigLifecycleDelegate{remove: sync})
}

func (n *listenConfigClientCache) Index(name string, indexer ListenConfigIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ListenConfig); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *listenConfigClientCache) GetIndexed(name, key string) ([]*ListenConfig, error) {
	var result []*ListenConfig
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ListenConfig); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *listenConfigClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type listenConfigLifecycleDelegate struct {
	create ListenConfigChangeHandlerFunc
	update ListenConfigChangeHandlerFunc
	remove ListenConfigChangeHandlerFunc
}

func (n *listenConfigLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *listenConfigLifecycleDelegate) Create(obj *ListenConfig) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *listenConfigLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *listenConfigLifecycleDelegate) Remove(obj *ListenConfig) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *listenConfigLifecycleDelegate) Updated(obj *ListenConfig) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
