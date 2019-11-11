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
	ClusterAuthTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterAuthToken",
	}
	ClusterAuthTokenResource = metav1.APIResource{
		Name:         "clusterauthtokens",
		SingularName: "clusterauthtoken",
		Namespaced:   true,

		Kind: ClusterAuthTokenGroupVersionKind.Kind,
	}

	ClusterAuthTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterauthtokens",
	}
)

func init() {
	resource.Put(ClusterAuthTokenGroupVersionResource)
}

func NewClusterAuthToken(namespace, name string, obj ClusterAuthToken) *ClusterAuthToken {
	obj.APIVersion, obj.Kind = ClusterAuthTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterAuthTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAuthToken `json:"items"`
}

type ClusterAuthTokenHandlerFunc func(key string, obj *ClusterAuthToken) (runtime.Object, error)

type ClusterAuthTokenChangeHandlerFunc func(obj *ClusterAuthToken) (runtime.Object, error)

type ClusterAuthTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterAuthToken, err error)
	Get(namespace, name string) (*ClusterAuthToken, error)
}

type ClusterAuthTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterAuthTokenLister
	AddHandler(ctx context.Context, name string, handler ClusterAuthTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAuthTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterAuthTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterAuthTokenHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterAuthTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterAuthToken) (*ClusterAuthToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAuthToken, error)
	Get(name string, opts metav1.GetOptions) (*ClusterAuthToken, error)
	Update(*ClusterAuthToken) (*ClusterAuthToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterAuthTokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterAuthTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterAuthTokenController
	AddHandler(ctx context.Context, name string, sync ClusterAuthTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAuthTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterAuthTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAuthTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAuthTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAuthTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAuthTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAuthTokenLifecycle)
}

type clusterAuthTokenLister struct {
	controller *clusterAuthTokenController
}

func (l *clusterAuthTokenLister) List(namespace string, selector labels.Selector) (ret []*ClusterAuthToken, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterAuthToken))
	})
	return
}

func (l *clusterAuthTokenLister) Get(namespace, name string) (*ClusterAuthToken, error) {
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
			Group:    ClusterAuthTokenGroupVersionKind.Group,
			Resource: "clusterAuthToken",
		}, key)
	}
	return obj.(*ClusterAuthToken), nil
}

type clusterAuthTokenController struct {
	controller.GenericController
}

func (c *clusterAuthTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterAuthTokenController) Lister() ClusterAuthTokenLister {
	return &clusterAuthTokenLister{
		controller: c,
	}
}

func (c *clusterAuthTokenController) AddHandler(ctx context.Context, name string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAuthToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAuthTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAuthToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAuthTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAuthToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAuthTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAuthToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterAuthTokenFactory struct {
}

func (c clusterAuthTokenFactory) Object() runtime.Object {
	return &ClusterAuthToken{}
}

func (c clusterAuthTokenFactory) List() runtime.Object {
	return &ClusterAuthTokenList{}
}

func (s *clusterAuthTokenClient) Controller() ClusterAuthTokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterAuthTokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterAuthTokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterAuthTokenController{
		GenericController: genericController,
	}

	s.client.clusterAuthTokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterAuthTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterAuthTokenController
}

func (s *clusterAuthTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterAuthTokenClient) Create(o *ClusterAuthToken) (*ClusterAuthToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) Get(name string, opts metav1.GetOptions) (*ClusterAuthToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAuthToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) Update(o *ClusterAuthToken) (*ClusterAuthToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterAuthTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterAuthTokenClient) List(opts metav1.ListOptions) (*ClusterAuthTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterAuthTokenList), err
}

func (s *clusterAuthTokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterAuthTokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterAuthTokenList), err
}

func (s *clusterAuthTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAuthTokenClient) Patch(o *ClusterAuthToken, patchType types.PatchType, data []byte, subresources ...string) (*ClusterAuthToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterAuthTokenClient) AddHandler(ctx context.Context, name string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAuthTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAuthTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAuthTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ClusterAuthTokenIndexer func(obj *ClusterAuthToken) ([]string, error)

type ClusterAuthTokenClientCache interface {
	Get(namespace, name string) (*ClusterAuthToken, error)
	List(namespace string, selector labels.Selector) ([]*ClusterAuthToken, error)

	Index(name string, indexer ClusterAuthTokenIndexer)
	GetIndexed(name, key string) ([]*ClusterAuthToken, error)
}

type ClusterAuthTokenClient interface {
	Create(*ClusterAuthToken) (*ClusterAuthToken, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ClusterAuthToken, error)
	Update(*ClusterAuthToken) (*ClusterAuthToken, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterAuthTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterAuthTokenClientCache

	OnCreate(ctx context.Context, name string, sync ClusterAuthTokenChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterAuthTokenChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterAuthTokenChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterAuthTokenInterface
}

type clusterAuthTokenClientCache struct {
	client *clusterAuthTokenClient2
}

type clusterAuthTokenClient2 struct {
	iface      ClusterAuthTokenInterface
	controller ClusterAuthTokenController
}

func (n *clusterAuthTokenClient2) Interface() ClusterAuthTokenInterface {
	return n.iface
}

func (n *clusterAuthTokenClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterAuthTokenClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterAuthTokenClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterAuthTokenClient2) Create(obj *ClusterAuthToken) (*ClusterAuthToken, error) {
	return n.iface.Create(obj)
}

func (n *clusterAuthTokenClient2) Get(namespace, name string, opts metav1.GetOptions) (*ClusterAuthToken, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterAuthTokenClient2) Update(obj *ClusterAuthToken) (*ClusterAuthToken, error) {
	return n.iface.Update(obj)
}

func (n *clusterAuthTokenClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterAuthTokenClient2) List(namespace string, opts metav1.ListOptions) (*ClusterAuthTokenList, error) {
	return n.iface.List(opts)
}

func (n *clusterAuthTokenClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterAuthTokenClientCache) Get(namespace, name string) (*ClusterAuthToken, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterAuthTokenClientCache) List(namespace string, selector labels.Selector) ([]*ClusterAuthToken, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterAuthTokenClient2) Cache() ClusterAuthTokenClientCache {
	n.loadController()
	return &clusterAuthTokenClientCache{
		client: n,
	}
}

func (n *clusterAuthTokenClient2) OnCreate(ctx context.Context, name string, sync ClusterAuthTokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterAuthTokenLifecycleDelegate{create: sync})
}

func (n *clusterAuthTokenClient2) OnChange(ctx context.Context, name string, sync ClusterAuthTokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterAuthTokenLifecycleDelegate{update: sync})
}

func (n *clusterAuthTokenClient2) OnRemove(ctx context.Context, name string, sync ClusterAuthTokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterAuthTokenLifecycleDelegate{remove: sync})
}

func (n *clusterAuthTokenClientCache) Index(name string, indexer ClusterAuthTokenIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ClusterAuthToken); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterAuthTokenClientCache) GetIndexed(name, key string) ([]*ClusterAuthToken, error) {
	var result []*ClusterAuthToken
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ClusterAuthToken); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterAuthTokenClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterAuthTokenLifecycleDelegate struct {
	create ClusterAuthTokenChangeHandlerFunc
	update ClusterAuthTokenChangeHandlerFunc
	remove ClusterAuthTokenChangeHandlerFunc
}

func (n *clusterAuthTokenLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterAuthTokenLifecycleDelegate) Create(obj *ClusterAuthToken) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterAuthTokenLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterAuthTokenLifecycleDelegate) Remove(obj *ClusterAuthToken) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterAuthTokenLifecycleDelegate) Updated(obj *ClusterAuthToken) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
