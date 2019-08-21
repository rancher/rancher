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
	NamespacedServiceAccountTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedServiceAccountToken",
	}
	NamespacedServiceAccountTokenResource = metav1.APIResource{
		Name:         "namespacedserviceaccounttokens",
		SingularName: "namespacedserviceaccounttoken",
		Namespaced:   true,

		Kind: NamespacedServiceAccountTokenGroupVersionKind.Kind,
	}

	NamespacedServiceAccountTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespacedserviceaccounttokens",
	}
)

func init() {
	resource.Put(NamespacedServiceAccountTokenGroupVersionResource)
}

func NewNamespacedServiceAccountToken(namespace, name string, obj NamespacedServiceAccountToken) *NamespacedServiceAccountToken {
	obj.APIVersion, obj.Kind = NamespacedServiceAccountTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedServiceAccountTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedServiceAccountToken `json:"items"`
}

type NamespacedServiceAccountTokenHandlerFunc func(key string, obj *NamespacedServiceAccountToken) (runtime.Object, error)

type NamespacedServiceAccountTokenChangeHandlerFunc func(obj *NamespacedServiceAccountToken) (runtime.Object, error)

type NamespacedServiceAccountTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedServiceAccountToken, err error)
	Get(namespace, name string) (*NamespacedServiceAccountToken, error)
}

type NamespacedServiceAccountTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedServiceAccountTokenLister
	AddHandler(ctx context.Context, name string, handler NamespacedServiceAccountTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedServiceAccountTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedServiceAccountTokenHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedServiceAccountTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error)
	Update(*NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedServiceAccountTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedServiceAccountTokenController
	AddHandler(ctx context.Context, name string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedServiceAccountTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedServiceAccountTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedServiceAccountTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedServiceAccountTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedServiceAccountTokenLifecycle)
}

type namespacedServiceAccountTokenLister struct {
	controller *namespacedServiceAccountTokenController
}

func (l *namespacedServiceAccountTokenLister) List(namespace string, selector labels.Selector) (ret []*NamespacedServiceAccountToken, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedServiceAccountToken))
	})
	return
}

func (l *namespacedServiceAccountTokenLister) Get(namespace, name string) (*NamespacedServiceAccountToken, error) {
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
			Group:    NamespacedServiceAccountTokenGroupVersionKind.Group,
			Resource: "namespacedServiceAccountToken",
		}, key)
	}
	return obj.(*NamespacedServiceAccountToken), nil
}

type namespacedServiceAccountTokenController struct {
	controller.GenericController
}

func (c *namespacedServiceAccountTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedServiceAccountTokenController) Lister() NamespacedServiceAccountTokenLister {
	return &namespacedServiceAccountTokenLister{
		controller: c,
	}
}

func (c *namespacedServiceAccountTokenController) AddHandler(ctx context.Context, name string, handler NamespacedServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedServiceAccountToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedServiceAccountTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedServiceAccountToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedServiceAccountTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedServiceAccountToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedServiceAccountTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedServiceAccountTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NamespacedServiceAccountToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedServiceAccountTokenFactory struct {
}

func (c namespacedServiceAccountTokenFactory) Object() runtime.Object {
	return &NamespacedServiceAccountToken{}
}

func (c namespacedServiceAccountTokenFactory) List() runtime.Object {
	return &NamespacedServiceAccountTokenList{}
}

func (s *namespacedServiceAccountTokenClient) Controller() NamespacedServiceAccountTokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedServiceAccountTokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedServiceAccountTokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedServiceAccountTokenController{
		GenericController: genericController,
	}

	s.client.namespacedServiceAccountTokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedServiceAccountTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedServiceAccountTokenController
}

func (s *namespacedServiceAccountTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedServiceAccountTokenClient) Create(o *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) Get(name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) Update(o *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedServiceAccountTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedServiceAccountTokenClient) List(opts metav1.ListOptions) (*NamespacedServiceAccountTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedServiceAccountTokenList), err
}

func (s *namespacedServiceAccountTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedServiceAccountTokenClient) Patch(o *NamespacedServiceAccountToken, patchType types.PatchType, data []byte, subresources ...string) (*NamespacedServiceAccountToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*NamespacedServiceAccountToken), err
}

func (s *namespacedServiceAccountTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedServiceAccountTokenClient) AddHandler(ctx context.Context, name string, sync NamespacedServiceAccountTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedServiceAccountTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedServiceAccountTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedServiceAccountTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedServiceAccountTokenLifecycle) {
	sync := NewNamespacedServiceAccountTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedServiceAccountTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedServiceAccountTokenLifecycle) {
	sync := NewNamespacedServiceAccountTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedServiceAccountTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedServiceAccountTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedServiceAccountTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedServiceAccountTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedServiceAccountTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedServiceAccountTokenLifecycle) {
	sync := NewNamespacedServiceAccountTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedServiceAccountTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedServiceAccountTokenLifecycle) {
	sync := NewNamespacedServiceAccountTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type NamespacedServiceAccountTokenIndexer func(obj *NamespacedServiceAccountToken) ([]string, error)

type NamespacedServiceAccountTokenClientCache interface {
	Get(namespace, name string) (*NamespacedServiceAccountToken, error)
	List(namespace string, selector labels.Selector) ([]*NamespacedServiceAccountToken, error)

	Index(name string, indexer NamespacedServiceAccountTokenIndexer)
	GetIndexed(name, key string) ([]*NamespacedServiceAccountToken, error)
}

type NamespacedServiceAccountTokenClient interface {
	Create(*NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	Get(namespace, name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error)
	Update(*NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*NamespacedServiceAccountTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() NamespacedServiceAccountTokenClientCache

	OnCreate(ctx context.Context, name string, sync NamespacedServiceAccountTokenChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync NamespacedServiceAccountTokenChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync NamespacedServiceAccountTokenChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() NamespacedServiceAccountTokenInterface
}

type namespacedServiceAccountTokenClientCache struct {
	client *namespacedServiceAccountTokenClient2
}

type namespacedServiceAccountTokenClient2 struct {
	iface      NamespacedServiceAccountTokenInterface
	controller NamespacedServiceAccountTokenController
}

func (n *namespacedServiceAccountTokenClient2) Interface() NamespacedServiceAccountTokenInterface {
	return n.iface
}

func (n *namespacedServiceAccountTokenClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *namespacedServiceAccountTokenClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *namespacedServiceAccountTokenClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *namespacedServiceAccountTokenClient2) Create(obj *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error) {
	return n.iface.Create(obj)
}

func (n *namespacedServiceAccountTokenClient2) Get(namespace, name string, opts metav1.GetOptions) (*NamespacedServiceAccountToken, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *namespacedServiceAccountTokenClient2) Update(obj *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error) {
	return n.iface.Update(obj)
}

func (n *namespacedServiceAccountTokenClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *namespacedServiceAccountTokenClient2) List(namespace string, opts metav1.ListOptions) (*NamespacedServiceAccountTokenList, error) {
	return n.iface.List(opts)
}

func (n *namespacedServiceAccountTokenClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *namespacedServiceAccountTokenClientCache) Get(namespace, name string) (*NamespacedServiceAccountToken, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *namespacedServiceAccountTokenClientCache) List(namespace string, selector labels.Selector) ([]*NamespacedServiceAccountToken, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *namespacedServiceAccountTokenClient2) Cache() NamespacedServiceAccountTokenClientCache {
	n.loadController()
	return &namespacedServiceAccountTokenClientCache{
		client: n,
	}
}

func (n *namespacedServiceAccountTokenClient2) OnCreate(ctx context.Context, name string, sync NamespacedServiceAccountTokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &namespacedServiceAccountTokenLifecycleDelegate{create: sync})
}

func (n *namespacedServiceAccountTokenClient2) OnChange(ctx context.Context, name string, sync NamespacedServiceAccountTokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &namespacedServiceAccountTokenLifecycleDelegate{update: sync})
}

func (n *namespacedServiceAccountTokenClient2) OnRemove(ctx context.Context, name string, sync NamespacedServiceAccountTokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &namespacedServiceAccountTokenLifecycleDelegate{remove: sync})
}

func (n *namespacedServiceAccountTokenClientCache) Index(name string, indexer NamespacedServiceAccountTokenIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*NamespacedServiceAccountToken); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *namespacedServiceAccountTokenClientCache) GetIndexed(name, key string) ([]*NamespacedServiceAccountToken, error) {
	var result []*NamespacedServiceAccountToken
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*NamespacedServiceAccountToken); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *namespacedServiceAccountTokenClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type namespacedServiceAccountTokenLifecycleDelegate struct {
	create NamespacedServiceAccountTokenChangeHandlerFunc
	update NamespacedServiceAccountTokenChangeHandlerFunc
	remove NamespacedServiceAccountTokenChangeHandlerFunc
}

func (n *namespacedServiceAccountTokenLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *namespacedServiceAccountTokenLifecycleDelegate) Create(obj *NamespacedServiceAccountToken) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *namespacedServiceAccountTokenLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *namespacedServiceAccountTokenLifecycleDelegate) Remove(obj *NamespacedServiceAccountToken) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *namespacedServiceAccountTokenLifecycleDelegate) Updated(obj *NamespacedServiceAccountToken) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
