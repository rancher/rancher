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
	NamespacedCertificateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedCertificate",
	}
	NamespacedCertificateResource = metav1.APIResource{
		Name:         "namespacedcertificates",
		SingularName: "namespacedcertificate",
		Namespaced:   true,

		Kind: NamespacedCertificateGroupVersionKind.Kind,
	}

	NamespacedCertificateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespacedcertificates",
	}
)

func init() {
	resource.Put(NamespacedCertificateGroupVersionResource)
}

func NewNamespacedCertificate(namespace, name string, obj NamespacedCertificate) *NamespacedCertificate {
	obj.APIVersion, obj.Kind = NamespacedCertificateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedCertificate `json:"items"`
}

type NamespacedCertificateHandlerFunc func(key string, obj *NamespacedCertificate) (runtime.Object, error)

type NamespacedCertificateChangeHandlerFunc func(obj *NamespacedCertificate) (runtime.Object, error)

type NamespacedCertificateLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedCertificate, err error)
	Get(namespace, name string) (*NamespacedCertificate, error)
}

type NamespacedCertificateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedCertificateLister
	AddHandler(ctx context.Context, name string, handler NamespacedCertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedCertificateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedCertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedCertificateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedCertificateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedCertificate) (*NamespacedCertificate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedCertificate, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedCertificate, error)
	Update(*NamespacedCertificate) (*NamespacedCertificate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedCertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedCertificateController
	AddHandler(ctx context.Context, name string, sync NamespacedCertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedCertificateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedCertificateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedCertificateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedCertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedCertificateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedCertificateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedCertificateLifecycle)
}

type namespacedCertificateLister struct {
	controller *namespacedCertificateController
}

func (l *namespacedCertificateLister) List(namespace string, selector labels.Selector) (ret []*NamespacedCertificate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedCertificate))
	})
	return
}

func (l *namespacedCertificateLister) Get(namespace, name string) (*NamespacedCertificate, error) {
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
			Group:    NamespacedCertificateGroupVersionKind.Group,
			Resource: "namespacedCertificate",
		}, key)
	}
	return obj.(*NamespacedCertificate), nil
}

type namespacedCertificateController struct {
	controller.GenericController
}

func (c *namespacedCertificateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedCertificateController) Lister() NamespacedCertificateLister {
	return &namespacedCertificateLister{
		controller: c,
	}
}

func (c *namespacedCertificateController) AddHandler(ctx context.Context, name string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedCertificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedCertificateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedCertificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedCertificateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedCertificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedCertificateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedCertificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedCertificateFactory struct {
}

func (c namespacedCertificateFactory) Object() runtime.Object {
	return &NamespacedCertificate{}
}

func (c namespacedCertificateFactory) List() runtime.Object {
	return &NamespacedCertificateList{}
}

func (s *namespacedCertificateClient) Controller() NamespacedCertificateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedCertificateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedCertificateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedCertificateController{
		GenericController: genericController,
	}

	s.client.namespacedCertificateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedCertificateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedCertificateController
}

func (s *namespacedCertificateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedCertificateClient) Create(o *NamespacedCertificate) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Get(name string, opts metav1.GetOptions) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Update(o *NamespacedCertificate) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedCertificateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedCertificateClient) List(opts metav1.ListOptions) (*NamespacedCertificateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedCertificateList), err
}

func (s *namespacedCertificateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedCertificateClient) Patch(o *NamespacedCertificate, patchType types.PatchType, data []byte, subresources ...string) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedCertificateClient) AddHandler(ctx context.Context, name string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedCertificateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedCertificateClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedCertificateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type NamespacedCertificateIndexer func(obj *NamespacedCertificate) ([]string, error)

type NamespacedCertificateClientCache interface {
	Get(namespace, name string) (*NamespacedCertificate, error)
	List(namespace string, selector labels.Selector) ([]*NamespacedCertificate, error)

	Index(name string, indexer NamespacedCertificateIndexer)
	GetIndexed(name, key string) ([]*NamespacedCertificate, error)
}

type NamespacedCertificateClient interface {
	Create(*NamespacedCertificate) (*NamespacedCertificate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*NamespacedCertificate, error)
	Update(*NamespacedCertificate) (*NamespacedCertificate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*NamespacedCertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() NamespacedCertificateClientCache

	OnCreate(ctx context.Context, name string, sync NamespacedCertificateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync NamespacedCertificateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync NamespacedCertificateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() NamespacedCertificateInterface
}

type namespacedCertificateClientCache struct {
	client *namespacedCertificateClient2
}

type namespacedCertificateClient2 struct {
	iface      NamespacedCertificateInterface
	controller NamespacedCertificateController
}

func (n *namespacedCertificateClient2) Interface() NamespacedCertificateInterface {
	return n.iface
}

func (n *namespacedCertificateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *namespacedCertificateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *namespacedCertificateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *namespacedCertificateClient2) Create(obj *NamespacedCertificate) (*NamespacedCertificate, error) {
	return n.iface.Create(obj)
}

func (n *namespacedCertificateClient2) Get(namespace, name string, opts metav1.GetOptions) (*NamespacedCertificate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *namespacedCertificateClient2) Update(obj *NamespacedCertificate) (*NamespacedCertificate, error) {
	return n.iface.Update(obj)
}

func (n *namespacedCertificateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *namespacedCertificateClient2) List(namespace string, opts metav1.ListOptions) (*NamespacedCertificateList, error) {
	return n.iface.List(opts)
}

func (n *namespacedCertificateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *namespacedCertificateClientCache) Get(namespace, name string) (*NamespacedCertificate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *namespacedCertificateClientCache) List(namespace string, selector labels.Selector) ([]*NamespacedCertificate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *namespacedCertificateClient2) Cache() NamespacedCertificateClientCache {
	n.loadController()
	return &namespacedCertificateClientCache{
		client: n,
	}
}

func (n *namespacedCertificateClient2) OnCreate(ctx context.Context, name string, sync NamespacedCertificateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &namespacedCertificateLifecycleDelegate{create: sync})
}

func (n *namespacedCertificateClient2) OnChange(ctx context.Context, name string, sync NamespacedCertificateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &namespacedCertificateLifecycleDelegate{update: sync})
}

func (n *namespacedCertificateClient2) OnRemove(ctx context.Context, name string, sync NamespacedCertificateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &namespacedCertificateLifecycleDelegate{remove: sync})
}

func (n *namespacedCertificateClientCache) Index(name string, indexer NamespacedCertificateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*NamespacedCertificate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *namespacedCertificateClientCache) GetIndexed(name, key string) ([]*NamespacedCertificate, error) {
	var result []*NamespacedCertificate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*NamespacedCertificate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *namespacedCertificateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type namespacedCertificateLifecycleDelegate struct {
	create NamespacedCertificateChangeHandlerFunc
	update NamespacedCertificateChangeHandlerFunc
	remove NamespacedCertificateChangeHandlerFunc
}

func (n *namespacedCertificateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *namespacedCertificateLifecycleDelegate) Create(obj *NamespacedCertificate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *namespacedCertificateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *namespacedCertificateLifecycleDelegate) Remove(obj *NamespacedCertificate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *namespacedCertificateLifecycleDelegate) Updated(obj *NamespacedCertificate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
