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
	DynamicSchemaGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DynamicSchema",
	}
	DynamicSchemaResource = metav1.APIResource{
		Name:         "dynamicschemas",
		SingularName: "dynamicschema",
		Namespaced:   false,
		Kind:         DynamicSchemaGroupVersionKind.Kind,
	}

	DynamicSchemaGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "dynamicschemas",
	}
)

func init() {
	resource.Put(DynamicSchemaGroupVersionResource)
}

func NewDynamicSchema(namespace, name string, obj DynamicSchema) *DynamicSchema {
	obj.APIVersion, obj.Kind = DynamicSchemaGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DynamicSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicSchema `json:"items"`
}

type DynamicSchemaHandlerFunc func(key string, obj *DynamicSchema) (runtime.Object, error)

type DynamicSchemaChangeHandlerFunc func(obj *DynamicSchema) (runtime.Object, error)

type DynamicSchemaLister interface {
	List(namespace string, selector labels.Selector) (ret []*DynamicSchema, err error)
	Get(namespace, name string) (*DynamicSchema, error)
}

type DynamicSchemaController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DynamicSchemaLister
	AddHandler(ctx context.Context, name string, handler DynamicSchemaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DynamicSchemaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler DynamicSchemaHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DynamicSchemaInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*DynamicSchema) (*DynamicSchema, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*DynamicSchema, error)
	Get(name string, opts metav1.GetOptions) (*DynamicSchema, error)
	Update(*DynamicSchema) (*DynamicSchema, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DynamicSchemaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DynamicSchemaController
	AddHandler(ctx context.Context, name string, sync DynamicSchemaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DynamicSchemaHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DynamicSchemaLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DynamicSchemaLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DynamicSchemaLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DynamicSchemaLifecycle)
}

type dynamicSchemaLister struct {
	controller *dynamicSchemaController
}

func (l *dynamicSchemaLister) List(namespace string, selector labels.Selector) (ret []*DynamicSchema, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*DynamicSchema))
	})
	return
}

func (l *dynamicSchemaLister) Get(namespace, name string) (*DynamicSchema, error) {
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
			Group:    DynamicSchemaGroupVersionKind.Group,
			Resource: "dynamicSchema",
		}, key)
	}
	return obj.(*DynamicSchema), nil
}

type dynamicSchemaController struct {
	controller.GenericController
}

func (c *dynamicSchemaController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *dynamicSchemaController) Lister() DynamicSchemaLister {
	return &dynamicSchemaLister{
		controller: c,
	}
}

func (c *dynamicSchemaController) AddHandler(ctx context.Context, name string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dynamicSchemaController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dynamicSchemaController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dynamicSchemaController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type dynamicSchemaFactory struct {
}

func (c dynamicSchemaFactory) Object() runtime.Object {
	return &DynamicSchema{}
}

func (c dynamicSchemaFactory) List() runtime.Object {
	return &DynamicSchemaList{}
}

func (s *dynamicSchemaClient) Controller() DynamicSchemaController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.dynamicSchemaControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DynamicSchemaGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &dynamicSchemaController{
		GenericController: genericController,
	}

	s.client.dynamicSchemaControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type dynamicSchemaClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DynamicSchemaController
}

func (s *dynamicSchemaClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *dynamicSchemaClient) Create(o *DynamicSchema) (*DynamicSchema, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Get(name string, opts metav1.GetOptions) (*DynamicSchema, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*DynamicSchema, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Update(o *DynamicSchema) (*DynamicSchema, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *dynamicSchemaClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *dynamicSchemaClient) List(opts metav1.ListOptions) (*DynamicSchemaList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DynamicSchemaList), err
}

func (s *dynamicSchemaClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *dynamicSchemaClient) Patch(o *DynamicSchema, patchType types.PatchType, data []byte, subresources ...string) (*DynamicSchema, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *dynamicSchemaClient) AddHandler(ctx context.Context, name string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *dynamicSchemaClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *dynamicSchemaClient) AddLifecycle(ctx context.Context, name string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *dynamicSchemaClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type DynamicSchemaIndexer func(obj *DynamicSchema) ([]string, error)

type DynamicSchemaClientCache interface {
	Get(namespace, name string) (*DynamicSchema, error)
	List(namespace string, selector labels.Selector) ([]*DynamicSchema, error)

	Index(name string, indexer DynamicSchemaIndexer)
	GetIndexed(name, key string) ([]*DynamicSchema, error)
}

type DynamicSchemaClient interface {
	Create(*DynamicSchema) (*DynamicSchema, error)
	Get(namespace, name string, opts metav1.GetOptions) (*DynamicSchema, error)
	Update(*DynamicSchema) (*DynamicSchema, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*DynamicSchemaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() DynamicSchemaClientCache

	OnCreate(ctx context.Context, name string, sync DynamicSchemaChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync DynamicSchemaChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync DynamicSchemaChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() DynamicSchemaInterface
}

type dynamicSchemaClientCache struct {
	client *dynamicSchemaClient2
}

type dynamicSchemaClient2 struct {
	iface      DynamicSchemaInterface
	controller DynamicSchemaController
}

func (n *dynamicSchemaClient2) Interface() DynamicSchemaInterface {
	return n.iface
}

func (n *dynamicSchemaClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *dynamicSchemaClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *dynamicSchemaClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *dynamicSchemaClient2) Create(obj *DynamicSchema) (*DynamicSchema, error) {
	return n.iface.Create(obj)
}

func (n *dynamicSchemaClient2) Get(namespace, name string, opts metav1.GetOptions) (*DynamicSchema, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *dynamicSchemaClient2) Update(obj *DynamicSchema) (*DynamicSchema, error) {
	return n.iface.Update(obj)
}

func (n *dynamicSchemaClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *dynamicSchemaClient2) List(namespace string, opts metav1.ListOptions) (*DynamicSchemaList, error) {
	return n.iface.List(opts)
}

func (n *dynamicSchemaClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *dynamicSchemaClientCache) Get(namespace, name string) (*DynamicSchema, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *dynamicSchemaClientCache) List(namespace string, selector labels.Selector) ([]*DynamicSchema, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *dynamicSchemaClient2) Cache() DynamicSchemaClientCache {
	n.loadController()
	return &dynamicSchemaClientCache{
		client: n,
	}
}

func (n *dynamicSchemaClient2) OnCreate(ctx context.Context, name string, sync DynamicSchemaChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &dynamicSchemaLifecycleDelegate{create: sync})
}

func (n *dynamicSchemaClient2) OnChange(ctx context.Context, name string, sync DynamicSchemaChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &dynamicSchemaLifecycleDelegate{update: sync})
}

func (n *dynamicSchemaClient2) OnRemove(ctx context.Context, name string, sync DynamicSchemaChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &dynamicSchemaLifecycleDelegate{remove: sync})
}

func (n *dynamicSchemaClientCache) Index(name string, indexer DynamicSchemaIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*DynamicSchema); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *dynamicSchemaClientCache) GetIndexed(name, key string) ([]*DynamicSchema, error) {
	var result []*DynamicSchema
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*DynamicSchema); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *dynamicSchemaClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type dynamicSchemaLifecycleDelegate struct {
	create DynamicSchemaChangeHandlerFunc
	update DynamicSchemaChangeHandlerFunc
	remove DynamicSchemaChangeHandlerFunc
}

func (n *dynamicSchemaLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *dynamicSchemaLifecycleDelegate) Create(obj *DynamicSchema) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *dynamicSchemaLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *dynamicSchemaLifecycleDelegate) Remove(obj *DynamicSchema) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *dynamicSchemaLifecycleDelegate) Updated(obj *DynamicSchema) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
