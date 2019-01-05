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
	CatalogTemplateVersionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CatalogTemplateVersion",
	}
	CatalogTemplateVersionResource = metav1.APIResource{
		Name:         "catalogtemplateversions",
		SingularName: "catalogtemplateversion",
		Namespaced:   true,

		Kind: CatalogTemplateVersionGroupVersionKind.Kind,
	}
)

func NewCatalogTemplateVersion(namespace, name string, obj CatalogTemplateVersion) *CatalogTemplateVersion {
	obj.APIVersion, obj.Kind = CatalogTemplateVersionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CatalogTemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CatalogTemplateVersion
}

type CatalogTemplateVersionHandlerFunc func(key string, obj *CatalogTemplateVersion) (runtime.Object, error)

type CatalogTemplateVersionChangeHandlerFunc func(obj *CatalogTemplateVersion) (runtime.Object, error)

type CatalogTemplateVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*CatalogTemplateVersion, err error)
	Get(namespace, name string) (*CatalogTemplateVersion, error)
}

type CatalogTemplateVersionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CatalogTemplateVersionLister
	AddHandler(ctx context.Context, name string, handler CatalogTemplateVersionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CatalogTemplateVersionHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CatalogTemplateVersionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error)
	Get(name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error)
	Update(*CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CatalogTemplateVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CatalogTemplateVersionController
	AddHandler(ctx context.Context, name string, sync CatalogTemplateVersionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateVersionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateVersionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateVersionLifecycle)
}

type catalogTemplateVersionLister struct {
	controller *catalogTemplateVersionController
}

func (l *catalogTemplateVersionLister) List(namespace string, selector labels.Selector) (ret []*CatalogTemplateVersion, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CatalogTemplateVersion))
	})
	return
}

func (l *catalogTemplateVersionLister) Get(namespace, name string) (*CatalogTemplateVersion, error) {
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
			Group:    CatalogTemplateVersionGroupVersionKind.Group,
			Resource: "catalogTemplateVersion",
		}, key)
	}
	return obj.(*CatalogTemplateVersion), nil
}

type catalogTemplateVersionController struct {
	controller.GenericController
}

func (c *catalogTemplateVersionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *catalogTemplateVersionController) Lister() CatalogTemplateVersionLister {
	return &catalogTemplateVersionLister{
		controller: c,
	}
}

func (c *catalogTemplateVersionController) AddHandler(ctx context.Context, name string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplateVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateVersionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type catalogTemplateVersionFactory struct {
}

func (c catalogTemplateVersionFactory) Object() runtime.Object {
	return &CatalogTemplateVersion{}
}

func (c catalogTemplateVersionFactory) List() runtime.Object {
	return &CatalogTemplateVersionList{}
}

func (s *catalogTemplateVersionClient) Controller() CatalogTemplateVersionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.catalogTemplateVersionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CatalogTemplateVersionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &catalogTemplateVersionController{
		GenericController: genericController,
	}

	s.client.catalogTemplateVersionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type catalogTemplateVersionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CatalogTemplateVersionController
}

func (s *catalogTemplateVersionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *catalogTemplateVersionClient) Create(o *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Get(name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Update(o *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogTemplateVersionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *catalogTemplateVersionClient) List(opts metav1.ListOptions) (*CatalogTemplateVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CatalogTemplateVersionList), err
}

func (s *catalogTemplateVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogTemplateVersionClient) Patch(o *CatalogTemplateVersion, patchType types.PatchType, data []byte, subresources ...string) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *catalogTemplateVersionClient) AddHandler(ctx context.Context, name string, sync CatalogTemplateVersionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateVersionClient) AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateVersionLifecycle) {
	sync := NewCatalogTemplateVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateVersionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateVersionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateVersionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateVersionLifecycle) {
	sync := NewCatalogTemplateVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type CatalogTemplateVersionIndexer func(obj *CatalogTemplateVersion) ([]string, error)

type CatalogTemplateVersionClientCache interface {
	Get(namespace, name string) (*CatalogTemplateVersion, error)
	List(namespace string, selector labels.Selector) ([]*CatalogTemplateVersion, error)

	Index(name string, indexer CatalogTemplateVersionIndexer)
	GetIndexed(name, key string) ([]*CatalogTemplateVersion, error)
}

type CatalogTemplateVersionClient interface {
	Create(*CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	Get(namespace, name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error)
	Update(*CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*CatalogTemplateVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() CatalogTemplateVersionClientCache

	OnCreate(ctx context.Context, name string, sync CatalogTemplateVersionChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync CatalogTemplateVersionChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync CatalogTemplateVersionChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() CatalogTemplateVersionInterface
}

type catalogTemplateVersionClientCache struct {
	client *catalogTemplateVersionClient2
}

type catalogTemplateVersionClient2 struct {
	iface      CatalogTemplateVersionInterface
	controller CatalogTemplateVersionController
}

func (n *catalogTemplateVersionClient2) Interface() CatalogTemplateVersionInterface {
	return n.iface
}

func (n *catalogTemplateVersionClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *catalogTemplateVersionClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *catalogTemplateVersionClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *catalogTemplateVersionClient2) Create(obj *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	return n.iface.Create(obj)
}

func (n *catalogTemplateVersionClient2) Get(namespace, name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *catalogTemplateVersionClient2) Update(obj *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	return n.iface.Update(obj)
}

func (n *catalogTemplateVersionClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *catalogTemplateVersionClient2) List(namespace string, opts metav1.ListOptions) (*CatalogTemplateVersionList, error) {
	return n.iface.List(opts)
}

func (n *catalogTemplateVersionClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *catalogTemplateVersionClientCache) Get(namespace, name string) (*CatalogTemplateVersion, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *catalogTemplateVersionClientCache) List(namespace string, selector labels.Selector) ([]*CatalogTemplateVersion, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *catalogTemplateVersionClient2) Cache() CatalogTemplateVersionClientCache {
	n.loadController()
	return &catalogTemplateVersionClientCache{
		client: n,
	}
}

func (n *catalogTemplateVersionClient2) OnCreate(ctx context.Context, name string, sync CatalogTemplateVersionChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &catalogTemplateVersionLifecycleDelegate{create: sync})
}

func (n *catalogTemplateVersionClient2) OnChange(ctx context.Context, name string, sync CatalogTemplateVersionChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &catalogTemplateVersionLifecycleDelegate{update: sync})
}

func (n *catalogTemplateVersionClient2) OnRemove(ctx context.Context, name string, sync CatalogTemplateVersionChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &catalogTemplateVersionLifecycleDelegate{remove: sync})
}

func (n *catalogTemplateVersionClientCache) Index(name string, indexer CatalogTemplateVersionIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*CatalogTemplateVersion); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *catalogTemplateVersionClientCache) GetIndexed(name, key string) ([]*CatalogTemplateVersion, error) {
	var result []*CatalogTemplateVersion
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*CatalogTemplateVersion); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *catalogTemplateVersionClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type catalogTemplateVersionLifecycleDelegate struct {
	create CatalogTemplateVersionChangeHandlerFunc
	update CatalogTemplateVersionChangeHandlerFunc
	remove CatalogTemplateVersionChangeHandlerFunc
}

func (n *catalogTemplateVersionLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *catalogTemplateVersionLifecycleDelegate) Create(obj *CatalogTemplateVersion) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *catalogTemplateVersionLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *catalogTemplateVersionLifecycleDelegate) Remove(obj *CatalogTemplateVersion) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *catalogTemplateVersionLifecycleDelegate) Updated(obj *CatalogTemplateVersion) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
