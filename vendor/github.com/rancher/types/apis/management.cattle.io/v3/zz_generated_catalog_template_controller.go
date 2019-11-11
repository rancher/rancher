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
	CatalogTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CatalogTemplate",
	}
	CatalogTemplateResource = metav1.APIResource{
		Name:         "catalogtemplates",
		SingularName: "catalogtemplate",
		Namespaced:   true,

		Kind: CatalogTemplateGroupVersionKind.Kind,
	}

	CatalogTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "catalogtemplates",
	}
)

func init() {
	resource.Put(CatalogTemplateGroupVersionResource)
}

func NewCatalogTemplate(namespace, name string, obj CatalogTemplate) *CatalogTemplate {
	obj.APIVersion, obj.Kind = CatalogTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CatalogTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CatalogTemplate `json:"items"`
}

type CatalogTemplateHandlerFunc func(key string, obj *CatalogTemplate) (runtime.Object, error)

type CatalogTemplateChangeHandlerFunc func(obj *CatalogTemplate) (runtime.Object, error)

type CatalogTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*CatalogTemplate, err error)
	Get(namespace, name string) (*CatalogTemplate, error)
}

type CatalogTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CatalogTemplateLister
	AddHandler(ctx context.Context, name string, handler CatalogTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CatalogTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CatalogTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CatalogTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CatalogTemplate) (*CatalogTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CatalogTemplate, error)
	Get(name string, opts metav1.GetOptions) (*CatalogTemplate, error)
	Update(*CatalogTemplate) (*CatalogTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CatalogTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*CatalogTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CatalogTemplateController
	AddHandler(ctx context.Context, name string, sync CatalogTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogTemplateLifecycle)
}

type catalogTemplateLister struct {
	controller *catalogTemplateController
}

func (l *catalogTemplateLister) List(namespace string, selector labels.Selector) (ret []*CatalogTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CatalogTemplate))
	})
	return
}

func (l *catalogTemplateLister) Get(namespace, name string) (*CatalogTemplate, error) {
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
			Group:    CatalogTemplateGroupVersionKind.Group,
			Resource: "catalogTemplate",
		}, key)
	}
	return obj.(*CatalogTemplate), nil
}

type catalogTemplateController struct {
	controller.GenericController
}

func (c *catalogTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *catalogTemplateController) Lister() CatalogTemplateLister {
	return &catalogTemplateLister{
		controller: c,
	}
}

func (c *catalogTemplateController) AddHandler(ctx context.Context, name string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type catalogTemplateFactory struct {
}

func (c catalogTemplateFactory) Object() runtime.Object {
	return &CatalogTemplate{}
}

func (c catalogTemplateFactory) List() runtime.Object {
	return &CatalogTemplateList{}
}

func (s *catalogTemplateClient) Controller() CatalogTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.catalogTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CatalogTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &catalogTemplateController{
		GenericController: genericController,
	}

	s.client.catalogTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type catalogTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CatalogTemplateController
}

func (s *catalogTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *catalogTemplateClient) Create(o *CatalogTemplate) (*CatalogTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CatalogTemplate), err
}

func (s *catalogTemplateClient) Get(name string, opts metav1.GetOptions) (*CatalogTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CatalogTemplate), err
}

func (s *catalogTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CatalogTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CatalogTemplate), err
}

func (s *catalogTemplateClient) Update(o *CatalogTemplate) (*CatalogTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CatalogTemplate), err
}

func (s *catalogTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *catalogTemplateClient) List(opts metav1.ListOptions) (*CatalogTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CatalogTemplateList), err
}

func (s *catalogTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*CatalogTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*CatalogTemplateList), err
}

func (s *catalogTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogTemplateClient) Patch(o *CatalogTemplate, patchType types.PatchType, data []byte, subresources ...string) (*CatalogTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*CatalogTemplate), err
}

func (s *catalogTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *catalogTemplateClient) AddHandler(ctx context.Context, name string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *catalogTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type CatalogTemplateIndexer func(obj *CatalogTemplate) ([]string, error)

type CatalogTemplateClientCache interface {
	Get(namespace, name string) (*CatalogTemplate, error)
	List(namespace string, selector labels.Selector) ([]*CatalogTemplate, error)

	Index(name string, indexer CatalogTemplateIndexer)
	GetIndexed(name, key string) ([]*CatalogTemplate, error)
}

type CatalogTemplateClient interface {
	Create(*CatalogTemplate) (*CatalogTemplate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*CatalogTemplate, error)
	Update(*CatalogTemplate) (*CatalogTemplate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*CatalogTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() CatalogTemplateClientCache

	OnCreate(ctx context.Context, name string, sync CatalogTemplateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync CatalogTemplateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync CatalogTemplateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() CatalogTemplateInterface
}

type catalogTemplateClientCache struct {
	client *catalogTemplateClient2
}

type catalogTemplateClient2 struct {
	iface      CatalogTemplateInterface
	controller CatalogTemplateController
}

func (n *catalogTemplateClient2) Interface() CatalogTemplateInterface {
	return n.iface
}

func (n *catalogTemplateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *catalogTemplateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *catalogTemplateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *catalogTemplateClient2) Create(obj *CatalogTemplate) (*CatalogTemplate, error) {
	return n.iface.Create(obj)
}

func (n *catalogTemplateClient2) Get(namespace, name string, opts metav1.GetOptions) (*CatalogTemplate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *catalogTemplateClient2) Update(obj *CatalogTemplate) (*CatalogTemplate, error) {
	return n.iface.Update(obj)
}

func (n *catalogTemplateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *catalogTemplateClient2) List(namespace string, opts metav1.ListOptions) (*CatalogTemplateList, error) {
	return n.iface.List(opts)
}

func (n *catalogTemplateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *catalogTemplateClientCache) Get(namespace, name string) (*CatalogTemplate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *catalogTemplateClientCache) List(namespace string, selector labels.Selector) ([]*CatalogTemplate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *catalogTemplateClient2) Cache() CatalogTemplateClientCache {
	n.loadController()
	return &catalogTemplateClientCache{
		client: n,
	}
}

func (n *catalogTemplateClient2) OnCreate(ctx context.Context, name string, sync CatalogTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &catalogTemplateLifecycleDelegate{create: sync})
}

func (n *catalogTemplateClient2) OnChange(ctx context.Context, name string, sync CatalogTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &catalogTemplateLifecycleDelegate{update: sync})
}

func (n *catalogTemplateClient2) OnRemove(ctx context.Context, name string, sync CatalogTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &catalogTemplateLifecycleDelegate{remove: sync})
}

func (n *catalogTemplateClientCache) Index(name string, indexer CatalogTemplateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*CatalogTemplate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *catalogTemplateClientCache) GetIndexed(name, key string) ([]*CatalogTemplate, error) {
	var result []*CatalogTemplate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*CatalogTemplate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *catalogTemplateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type catalogTemplateLifecycleDelegate struct {
	create CatalogTemplateChangeHandlerFunc
	update CatalogTemplateChangeHandlerFunc
	remove CatalogTemplateChangeHandlerFunc
}

func (n *catalogTemplateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *catalogTemplateLifecycleDelegate) Create(obj *CatalogTemplate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *catalogTemplateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *catalogTemplateLifecycleDelegate) Remove(obj *CatalogTemplate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *catalogTemplateLifecycleDelegate) Updated(obj *CatalogTemplate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
