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
	CatalogGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Catalog",
	}
	CatalogResource = metav1.APIResource{
		Name:         "catalogs",
		SingularName: "catalog",
		Namespaced:   false,
		Kind:         CatalogGroupVersionKind.Kind,
	}

	CatalogGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "catalogs",
	}
)

func init() {
	resource.Put(CatalogGroupVersionResource)
}

func NewCatalog(namespace, name string, obj Catalog) *Catalog {
	obj.APIVersion, obj.Kind = CatalogGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Catalog `json:"items"`
}

type CatalogHandlerFunc func(key string, obj *Catalog) (runtime.Object, error)

type CatalogChangeHandlerFunc func(obj *Catalog) (runtime.Object, error)

type CatalogLister interface {
	List(namespace string, selector labels.Selector) (ret []*Catalog, err error)
	Get(namespace, name string) (*Catalog, error)
}

type CatalogController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CatalogLister
	AddHandler(ctx context.Context, name string, handler CatalogHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CatalogHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CatalogHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CatalogInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Catalog) (*Catalog, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Catalog, error)
	Get(name string, opts metav1.GetOptions) (*Catalog, error)
	Update(*Catalog) (*Catalog, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CatalogController
	AddHandler(ctx context.Context, name string, sync CatalogHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CatalogLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogLifecycle)
}

type catalogLister struct {
	controller *catalogController
}

func (l *catalogLister) List(namespace string, selector labels.Selector) (ret []*Catalog, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Catalog))
	})
	return
}

func (l *catalogLister) Get(namespace, name string) (*Catalog, error) {
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
			Group:    CatalogGroupVersionKind.Group,
			Resource: "catalog",
		}, key)
	}
	return obj.(*Catalog), nil
}

type catalogController struct {
	controller.GenericController
}

func (c *catalogController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *catalogController) Lister() CatalogLister {
	return &catalogLister{
		controller: c,
	}
}

func (c *catalogController) AddHandler(ctx context.Context, name string, handler CatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Catalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Catalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Catalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Catalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type catalogFactory struct {
}

func (c catalogFactory) Object() runtime.Object {
	return &Catalog{}
}

func (c catalogFactory) List() runtime.Object {
	return &CatalogList{}
}

func (s *catalogClient) Controller() CatalogController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.catalogControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CatalogGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &catalogController{
		GenericController: genericController,
	}

	s.client.catalogControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type catalogClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CatalogController
}

func (s *catalogClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *catalogClient) Create(o *Catalog) (*Catalog, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Catalog), err
}

func (s *catalogClient) Get(name string, opts metav1.GetOptions) (*Catalog, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Catalog), err
}

func (s *catalogClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Catalog, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Catalog), err
}

func (s *catalogClient) Update(o *Catalog) (*Catalog, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Catalog), err
}

func (s *catalogClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *catalogClient) List(opts metav1.ListOptions) (*CatalogList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CatalogList), err
}

func (s *catalogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogClient) Patch(o *Catalog, patchType types.PatchType, data []byte, subresources ...string) (*Catalog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Catalog), err
}

func (s *catalogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *catalogClient) AddHandler(ctx context.Context, name string, sync CatalogHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogClient) AddLifecycle(ctx context.Context, name string, lifecycle CatalogLifecycle) {
	sync := NewCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogLifecycle) {
	sync := NewCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *catalogClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogLifecycle) {
	sync := NewCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogLifecycle) {
	sync := NewCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type CatalogIndexer func(obj *Catalog) ([]string, error)

type CatalogClientCache interface {
	Get(namespace, name string) (*Catalog, error)
	List(namespace string, selector labels.Selector) ([]*Catalog, error)

	Index(name string, indexer CatalogIndexer)
	GetIndexed(name, key string) ([]*Catalog, error)
}

type CatalogClient interface {
	Create(*Catalog) (*Catalog, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Catalog, error)
	Update(*Catalog) (*Catalog, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*CatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() CatalogClientCache

	OnCreate(ctx context.Context, name string, sync CatalogChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync CatalogChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync CatalogChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() CatalogInterface
}

type catalogClientCache struct {
	client *catalogClient2
}

type catalogClient2 struct {
	iface      CatalogInterface
	controller CatalogController
}

func (n *catalogClient2) Interface() CatalogInterface {
	return n.iface
}

func (n *catalogClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *catalogClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *catalogClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *catalogClient2) Create(obj *Catalog) (*Catalog, error) {
	return n.iface.Create(obj)
}

func (n *catalogClient2) Get(namespace, name string, opts metav1.GetOptions) (*Catalog, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *catalogClient2) Update(obj *Catalog) (*Catalog, error) {
	return n.iface.Update(obj)
}

func (n *catalogClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *catalogClient2) List(namespace string, opts metav1.ListOptions) (*CatalogList, error) {
	return n.iface.List(opts)
}

func (n *catalogClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *catalogClientCache) Get(namespace, name string) (*Catalog, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *catalogClientCache) List(namespace string, selector labels.Selector) ([]*Catalog, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *catalogClient2) Cache() CatalogClientCache {
	n.loadController()
	return &catalogClientCache{
		client: n,
	}
}

func (n *catalogClient2) OnCreate(ctx context.Context, name string, sync CatalogChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &catalogLifecycleDelegate{create: sync})
}

func (n *catalogClient2) OnChange(ctx context.Context, name string, sync CatalogChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &catalogLifecycleDelegate{update: sync})
}

func (n *catalogClient2) OnRemove(ctx context.Context, name string, sync CatalogChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &catalogLifecycleDelegate{remove: sync})
}

func (n *catalogClientCache) Index(name string, indexer CatalogIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Catalog); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *catalogClientCache) GetIndexed(name, key string) ([]*Catalog, error) {
	var result []*Catalog
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Catalog); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *catalogClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type catalogLifecycleDelegate struct {
	create CatalogChangeHandlerFunc
	update CatalogChangeHandlerFunc
	remove CatalogChangeHandlerFunc
}

func (n *catalogLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *catalogLifecycleDelegate) Create(obj *Catalog) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *catalogLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *catalogLifecycleDelegate) Remove(obj *Catalog) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *catalogLifecycleDelegate) Updated(obj *Catalog) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
