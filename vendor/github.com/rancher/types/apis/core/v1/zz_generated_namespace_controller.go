package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v1 "k8s.io/api/core/v1"
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
	NamespaceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Namespace",
	}
	NamespaceResource = metav1.APIResource{
		Name:         "namespaces",
		SingularName: "namespace",
		Namespaced:   false,
		Kind:         NamespaceGroupVersionKind.Kind,
	}

	NamespaceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespaces",
	}
)

func init() {
	resource.Put(NamespaceGroupVersionResource)
}

func NewNamespace(namespace, name string, obj v1.Namespace) *v1.Namespace {
	obj.APIVersion, obj.Kind = NamespaceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Namespace `json:"items"`
}

type NamespaceHandlerFunc func(key string, obj *v1.Namespace) (runtime.Object, error)

type NamespaceChangeHandlerFunc func(obj *v1.Namespace) (runtime.Object, error)

type NamespaceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Namespace, err error)
	Get(namespace, name string) (*v1.Namespace, error)
}

type NamespaceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespaceLister
	AddHandler(ctx context.Context, name string, handler NamespaceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespaceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespaceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespaceHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespaceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Namespace) (*v1.Namespace, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Namespace, error)
	Get(name string, opts metav1.GetOptions) (*v1.Namespace, error)
	Update(*v1.Namespace) (*v1.Namespace, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespaceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*NamespaceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespaceController
	AddHandler(ctx context.Context, name string, sync NamespaceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespaceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespaceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespaceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespaceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespaceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespaceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespaceLifecycle)
}

type namespaceLister struct {
	controller *namespaceController
}

func (l *namespaceLister) List(namespace string, selector labels.Selector) (ret []*v1.Namespace, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Namespace))
	})
	return
}

func (l *namespaceLister) Get(namespace, name string) (*v1.Namespace, error) {
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
			Group:    NamespaceGroupVersionKind.Group,
			Resource: "namespace",
		}, key)
	}
	return obj.(*v1.Namespace), nil
}

type namespaceController struct {
	controller.GenericController
}

func (c *namespaceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespaceController) Lister() NamespaceLister {
	return &namespaceLister{
		controller: c,
	}
}

func (c *namespaceController) AddHandler(ctx context.Context, name string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespaceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespaceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespaceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Namespace); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespaceFactory struct {
}

func (c namespaceFactory) Object() runtime.Object {
	return &v1.Namespace{}
}

func (c namespaceFactory) List() runtime.Object {
	return &NamespaceList{}
}

func (s *namespaceClient) Controller() NamespaceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespaceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespaceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespaceController{
		GenericController: genericController,
	}

	s.client.namespaceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespaceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespaceController
}

func (s *namespaceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespaceClient) Create(o *v1.Namespace) (*v1.Namespace, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Get(name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Update(o *v1.Namespace) (*v1.Namespace, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespaceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespaceClient) List(opts metav1.ListOptions) (*NamespaceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespaceList), err
}

func (s *namespaceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*NamespaceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*NamespaceList), err
}

func (s *namespaceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespaceClient) Patch(o *v1.Namespace, patchType types.PatchType, data []byte, subresources ...string) (*v1.Namespace, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Namespace), err
}

func (s *namespaceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespaceClient) AddHandler(ctx context.Context, name string, sync NamespaceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespaceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespaceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespaceClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespaceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespaceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespaceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespaceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespaceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespaceLifecycle) {
	sync := NewNamespaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type NamespaceIndexer func(obj *v1.Namespace) ([]string, error)

type NamespaceClientCache interface {
	Get(namespace, name string) (*v1.Namespace, error)
	List(namespace string, selector labels.Selector) ([]*v1.Namespace, error)

	Index(name string, indexer NamespaceIndexer)
	GetIndexed(name, key string) ([]*v1.Namespace, error)
}

type NamespaceClient interface {
	Create(*v1.Namespace) (*v1.Namespace, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Namespace, error)
	Update(*v1.Namespace) (*v1.Namespace, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*NamespaceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() NamespaceClientCache

	OnCreate(ctx context.Context, name string, sync NamespaceChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync NamespaceChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync NamespaceChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() NamespaceInterface
}

type namespaceClientCache struct {
	client *namespaceClient2
}

type namespaceClient2 struct {
	iface      NamespaceInterface
	controller NamespaceController
}

func (n *namespaceClient2) Interface() NamespaceInterface {
	return n.iface
}

func (n *namespaceClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *namespaceClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *namespaceClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *namespaceClient2) Create(obj *v1.Namespace) (*v1.Namespace, error) {
	return n.iface.Create(obj)
}

func (n *namespaceClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *namespaceClient2) Update(obj *v1.Namespace) (*v1.Namespace, error) {
	return n.iface.Update(obj)
}

func (n *namespaceClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *namespaceClient2) List(namespace string, opts metav1.ListOptions) (*NamespaceList, error) {
	return n.iface.List(opts)
}

func (n *namespaceClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *namespaceClientCache) Get(namespace, name string) (*v1.Namespace, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *namespaceClientCache) List(namespace string, selector labels.Selector) ([]*v1.Namespace, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *namespaceClient2) Cache() NamespaceClientCache {
	n.loadController()
	return &namespaceClientCache{
		client: n,
	}
}

func (n *namespaceClient2) OnCreate(ctx context.Context, name string, sync NamespaceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &namespaceLifecycleDelegate{create: sync})
}

func (n *namespaceClient2) OnChange(ctx context.Context, name string, sync NamespaceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &namespaceLifecycleDelegate{update: sync})
}

func (n *namespaceClient2) OnRemove(ctx context.Context, name string, sync NamespaceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &namespaceLifecycleDelegate{remove: sync})
}

func (n *namespaceClientCache) Index(name string, indexer NamespaceIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Namespace); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *namespaceClientCache) GetIndexed(name, key string) ([]*v1.Namespace, error) {
	var result []*v1.Namespace
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Namespace); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *namespaceClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type namespaceLifecycleDelegate struct {
	create NamespaceChangeHandlerFunc
	update NamespaceChangeHandlerFunc
	remove NamespaceChangeHandlerFunc
}

func (n *namespaceLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *namespaceLifecycleDelegate) Create(obj *v1.Namespace) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *namespaceLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *namespaceLifecycleDelegate) Remove(obj *v1.Namespace) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *namespaceLifecycleDelegate) Updated(obj *v1.Namespace) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
