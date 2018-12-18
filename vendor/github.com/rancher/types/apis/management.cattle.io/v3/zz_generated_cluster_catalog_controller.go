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
	ClusterCatalogGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterCatalog",
	}
	ClusterCatalogResource = metav1.APIResource{
		Name:         "clustercatalogs",
		SingularName: "clustercatalog",
		Namespaced:   true,

		Kind: ClusterCatalogGroupVersionKind.Kind,
	}
)

func NewClusterCatalog(namespace, name string, obj ClusterCatalog) *ClusterCatalog {
	obj.APIVersion, obj.Kind = ClusterCatalogGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCatalog
}

type ClusterCatalogHandlerFunc func(key string, obj *ClusterCatalog) (runtime.Object, error)

type ClusterCatalogChangeHandlerFunc func(obj *ClusterCatalog) (runtime.Object, error)

type ClusterCatalogLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterCatalog, err error)
	Get(namespace, name string) (*ClusterCatalog, error)
}

type ClusterCatalogController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterCatalogLister
	AddHandler(ctx context.Context, name string, handler ClusterCatalogHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterCatalogHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterCatalogInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterCatalog) (*ClusterCatalog, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterCatalog, error)
	Get(name string, opts metav1.GetOptions) (*ClusterCatalog, error)
	Update(*ClusterCatalog) (*ClusterCatalog, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterCatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterCatalogController
	AddHandler(ctx context.Context, name string, sync ClusterCatalogHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterCatalogLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterCatalogHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterCatalogLifecycle)
}

type clusterCatalogLister struct {
	controller *clusterCatalogController
}

func (l *clusterCatalogLister) List(namespace string, selector labels.Selector) (ret []*ClusterCatalog, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterCatalog))
	})
	return
}

func (l *clusterCatalogLister) Get(namespace, name string) (*ClusterCatalog, error) {
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
			Group:    ClusterCatalogGroupVersionKind.Group,
			Resource: "clusterCatalog",
		}, key)
	}
	return obj.(*ClusterCatalog), nil
}

type clusterCatalogController struct {
	controller.GenericController
}

func (c *clusterCatalogController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterCatalogController) Lister() ClusterCatalogLister {
	return &clusterCatalogLister{
		controller: c,
	}
}

func (c *clusterCatalogController) AddHandler(ctx context.Context, name string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterCatalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterCatalogController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterCatalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterCatalogFactory struct {
}

func (c clusterCatalogFactory) Object() runtime.Object {
	return &ClusterCatalog{}
}

func (c clusterCatalogFactory) List() runtime.Object {
	return &ClusterCatalogList{}
}

func (s *clusterCatalogClient) Controller() ClusterCatalogController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterCatalogControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterCatalogGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterCatalogController{
		GenericController: genericController,
	}

	s.client.clusterCatalogControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterCatalogClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterCatalogController
}

func (s *clusterCatalogClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterCatalogClient) Create(o *ClusterCatalog) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) Get(name string, opts metav1.GetOptions) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterCatalog, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) Update(o *ClusterCatalog) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterCatalogClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterCatalogClient) List(opts metav1.ListOptions) (*ClusterCatalogList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterCatalogList), err
}

func (s *clusterCatalogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterCatalogClient) Patch(o *ClusterCatalog, patchType types.PatchType, data []byte, subresources ...string) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterCatalogClient) AddHandler(ctx context.Context, name string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterCatalogClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterCatalogClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterCatalogClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type ClusterCatalogIndexer func(obj *ClusterCatalog) ([]string, error)

type ClusterCatalogClientCache interface {
	Get(namespace, name string) (*ClusterCatalog, error)
	List(namespace string, selector labels.Selector) ([]*ClusterCatalog, error)

	Index(name string, indexer ClusterCatalogIndexer)
	GetIndexed(name, key string) ([]*ClusterCatalog, error)
}

type ClusterCatalogClient interface {
	Create(*ClusterCatalog) (*ClusterCatalog, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ClusterCatalog, error)
	Update(*ClusterCatalog) (*ClusterCatalog, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterCatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterCatalogClientCache

	OnCreate(ctx context.Context, name string, sync ClusterCatalogChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterCatalogChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterCatalogChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterCatalogInterface
}

type clusterCatalogClientCache struct {
	client *clusterCatalogClient2
}

type clusterCatalogClient2 struct {
	iface      ClusterCatalogInterface
	controller ClusterCatalogController
}

func (n *clusterCatalogClient2) Interface() ClusterCatalogInterface {
	return n.iface
}

func (n *clusterCatalogClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterCatalogClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterCatalogClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterCatalogClient2) Create(obj *ClusterCatalog) (*ClusterCatalog, error) {
	return n.iface.Create(obj)
}

func (n *clusterCatalogClient2) Get(namespace, name string, opts metav1.GetOptions) (*ClusterCatalog, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterCatalogClient2) Update(obj *ClusterCatalog) (*ClusterCatalog, error) {
	return n.iface.Update(obj)
}

func (n *clusterCatalogClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterCatalogClient2) List(namespace string, opts metav1.ListOptions) (*ClusterCatalogList, error) {
	return n.iface.List(opts)
}

func (n *clusterCatalogClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterCatalogClientCache) Get(namespace, name string) (*ClusterCatalog, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterCatalogClientCache) List(namespace string, selector labels.Selector) ([]*ClusterCatalog, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterCatalogClient2) Cache() ClusterCatalogClientCache {
	n.loadController()
	return &clusterCatalogClientCache{
		client: n,
	}
}

func (n *clusterCatalogClient2) OnCreate(ctx context.Context, name string, sync ClusterCatalogChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterCatalogLifecycleDelegate{create: sync})
}

func (n *clusterCatalogClient2) OnChange(ctx context.Context, name string, sync ClusterCatalogChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterCatalogLifecycleDelegate{update: sync})
}

func (n *clusterCatalogClient2) OnRemove(ctx context.Context, name string, sync ClusterCatalogChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterCatalogLifecycleDelegate{remove: sync})
}

func (n *clusterCatalogClientCache) Index(name string, indexer ClusterCatalogIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ClusterCatalog); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterCatalogClientCache) GetIndexed(name, key string) ([]*ClusterCatalog, error) {
	var result []*ClusterCatalog
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ClusterCatalog); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterCatalogClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterCatalogLifecycleDelegate struct {
	create ClusterCatalogChangeHandlerFunc
	update ClusterCatalogChangeHandlerFunc
	remove ClusterCatalogChangeHandlerFunc
}

func (n *clusterCatalogLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterCatalogLifecycleDelegate) Create(obj *ClusterCatalog) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterCatalogLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterCatalogLifecycleDelegate) Remove(obj *ClusterCatalog) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterCatalogLifecycleDelegate) Updated(obj *ClusterCatalog) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
