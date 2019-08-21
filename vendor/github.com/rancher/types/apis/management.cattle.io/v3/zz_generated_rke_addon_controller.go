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
	RKEAddonGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RKEAddon",
	}
	RKEAddonResource = metav1.APIResource{
		Name:         "rkeaddons",
		SingularName: "rkeaddon",
		Namespaced:   true,

		Kind: RKEAddonGroupVersionKind.Kind,
	}

	RKEAddonGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rkeaddons",
	}
)

func init() {
	resource.Put(RKEAddonGroupVersionResource)
}

func NewRKEAddon(namespace, name string, obj RKEAddon) *RKEAddon {
	obj.APIVersion, obj.Kind = RKEAddonGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RKEAddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RKEAddon `json:"items"`
}

type RKEAddonHandlerFunc func(key string, obj *RKEAddon) (runtime.Object, error)

type RKEAddonChangeHandlerFunc func(obj *RKEAddon) (runtime.Object, error)

type RKEAddonLister interface {
	List(namespace string, selector labels.Selector) (ret []*RKEAddon, err error)
	Get(namespace, name string) (*RKEAddon, error)
}

type RKEAddonController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RKEAddonLister
	AddHandler(ctx context.Context, name string, handler RKEAddonHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEAddonHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RKEAddonHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RKEAddonHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RKEAddonInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*RKEAddon) (*RKEAddon, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEAddon, error)
	Get(name string, opts metav1.GetOptions) (*RKEAddon, error)
	Update(*RKEAddon) (*RKEAddon, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RKEAddonList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RKEAddonController
	AddHandler(ctx context.Context, name string, sync RKEAddonHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEAddonHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RKEAddonLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEAddonLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEAddonHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEAddonHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEAddonLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEAddonLifecycle)
}

type rkeAddonLister struct {
	controller *rkeAddonController
}

func (l *rkeAddonLister) List(namespace string, selector labels.Selector) (ret []*RKEAddon, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*RKEAddon))
	})
	return
}

func (l *rkeAddonLister) Get(namespace, name string) (*RKEAddon, error) {
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
			Group:    RKEAddonGroupVersionKind.Group,
			Resource: "rkeAddon",
		}, key)
	}
	return obj.(*RKEAddon), nil
}

type rkeAddonController struct {
	controller.GenericController
}

func (c *rkeAddonController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rkeAddonController) Lister() RKEAddonLister {
	return &rkeAddonLister{
		controller: c,
	}
}

func (c *rkeAddonController) AddHandler(ctx context.Context, name string, handler RKEAddonHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEAddon); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeAddonController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RKEAddonHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEAddon); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeAddonController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RKEAddonHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEAddon); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeAddonController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RKEAddonHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEAddon); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rkeAddonFactory struct {
}

func (c rkeAddonFactory) Object() runtime.Object {
	return &RKEAddon{}
}

func (c rkeAddonFactory) List() runtime.Object {
	return &RKEAddonList{}
}

func (s *rkeAddonClient) Controller() RKEAddonController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.rkeAddonControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RKEAddonGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &rkeAddonController{
		GenericController: genericController,
	}

	s.client.rkeAddonControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type rkeAddonClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RKEAddonController
}

func (s *rkeAddonClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rkeAddonClient) Create(o *RKEAddon) (*RKEAddon, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*RKEAddon), err
}

func (s *rkeAddonClient) Get(name string, opts metav1.GetOptions) (*RKEAddon, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*RKEAddon), err
}

func (s *rkeAddonClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEAddon, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*RKEAddon), err
}

func (s *rkeAddonClient) Update(o *RKEAddon) (*RKEAddon, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*RKEAddon), err
}

func (s *rkeAddonClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rkeAddonClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rkeAddonClient) List(opts metav1.ListOptions) (*RKEAddonList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RKEAddonList), err
}

func (s *rkeAddonClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rkeAddonClient) Patch(o *RKEAddon, patchType types.PatchType, data []byte, subresources ...string) (*RKEAddon, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*RKEAddon), err
}

func (s *rkeAddonClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rkeAddonClient) AddHandler(ctx context.Context, name string, sync RKEAddonHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeAddonClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEAddonHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeAddonClient) AddLifecycle(ctx context.Context, name string, lifecycle RKEAddonLifecycle) {
	sync := NewRKEAddonLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeAddonClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEAddonLifecycle) {
	sync := NewRKEAddonLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeAddonClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEAddonHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeAddonClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEAddonHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rkeAddonClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEAddonLifecycle) {
	sync := NewRKEAddonLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeAddonClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEAddonLifecycle) {
	sync := NewRKEAddonLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type RKEAddonIndexer func(obj *RKEAddon) ([]string, error)

type RKEAddonClientCache interface {
	Get(namespace, name string) (*RKEAddon, error)
	List(namespace string, selector labels.Selector) ([]*RKEAddon, error)

	Index(name string, indexer RKEAddonIndexer)
	GetIndexed(name, key string) ([]*RKEAddon, error)
}

type RKEAddonClient interface {
	Create(*RKEAddon) (*RKEAddon, error)
	Get(namespace, name string, opts metav1.GetOptions) (*RKEAddon, error)
	Update(*RKEAddon) (*RKEAddon, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*RKEAddonList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() RKEAddonClientCache

	OnCreate(ctx context.Context, name string, sync RKEAddonChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync RKEAddonChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync RKEAddonChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() RKEAddonInterface
}

type rkeAddonClientCache struct {
	client *rkeAddonClient2
}

type rkeAddonClient2 struct {
	iface      RKEAddonInterface
	controller RKEAddonController
}

func (n *rkeAddonClient2) Interface() RKEAddonInterface {
	return n.iface
}

func (n *rkeAddonClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *rkeAddonClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *rkeAddonClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *rkeAddonClient2) Create(obj *RKEAddon) (*RKEAddon, error) {
	return n.iface.Create(obj)
}

func (n *rkeAddonClient2) Get(namespace, name string, opts metav1.GetOptions) (*RKEAddon, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *rkeAddonClient2) Update(obj *RKEAddon) (*RKEAddon, error) {
	return n.iface.Update(obj)
}

func (n *rkeAddonClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *rkeAddonClient2) List(namespace string, opts metav1.ListOptions) (*RKEAddonList, error) {
	return n.iface.List(opts)
}

func (n *rkeAddonClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *rkeAddonClientCache) Get(namespace, name string) (*RKEAddon, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *rkeAddonClientCache) List(namespace string, selector labels.Selector) ([]*RKEAddon, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *rkeAddonClient2) Cache() RKEAddonClientCache {
	n.loadController()
	return &rkeAddonClientCache{
		client: n,
	}
}

func (n *rkeAddonClient2) OnCreate(ctx context.Context, name string, sync RKEAddonChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &rkeAddonLifecycleDelegate{create: sync})
}

func (n *rkeAddonClient2) OnChange(ctx context.Context, name string, sync RKEAddonChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &rkeAddonLifecycleDelegate{update: sync})
}

func (n *rkeAddonClient2) OnRemove(ctx context.Context, name string, sync RKEAddonChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &rkeAddonLifecycleDelegate{remove: sync})
}

func (n *rkeAddonClientCache) Index(name string, indexer RKEAddonIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*RKEAddon); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *rkeAddonClientCache) GetIndexed(name, key string) ([]*RKEAddon, error) {
	var result []*RKEAddon
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*RKEAddon); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *rkeAddonClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type rkeAddonLifecycleDelegate struct {
	create RKEAddonChangeHandlerFunc
	update RKEAddonChangeHandlerFunc
	remove RKEAddonChangeHandlerFunc
}

func (n *rkeAddonLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *rkeAddonLifecycleDelegate) Create(obj *RKEAddon) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *rkeAddonLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *rkeAddonLifecycleDelegate) Remove(obj *RKEAddon) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *rkeAddonLifecycleDelegate) Updated(obj *RKEAddon) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
