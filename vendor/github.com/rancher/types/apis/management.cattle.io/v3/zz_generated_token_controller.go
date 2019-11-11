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
	TokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Token",
	}
	TokenResource = metav1.APIResource{
		Name:         "tokens",
		SingularName: "token",
		Namespaced:   false,
		Kind:         TokenGroupVersionKind.Kind,
	}

	TokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "tokens",
	}
)

func init() {
	resource.Put(TokenGroupVersionResource)
}

func NewToken(namespace, name string, obj Token) *Token {
	obj.APIVersion, obj.Kind = TokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Token `json:"items"`
}

type TokenHandlerFunc func(key string, obj *Token) (runtime.Object, error)

type TokenChangeHandlerFunc func(obj *Token) (runtime.Object, error)

type TokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*Token, err error)
	Get(namespace, name string) (*Token, error)
}

type TokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TokenLister
	AddHandler(ctx context.Context, name string, handler TokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler TokenHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Token) (*Token, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Token, error)
	Get(name string, opts metav1.GetOptions) (*Token, error)
	Update(*Token) (*Token, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*TokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TokenController
	AddHandler(ctx context.Context, name string, sync TokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TokenLifecycle)
}

type tokenLister struct {
	controller *tokenController
}

func (l *tokenLister) List(namespace string, selector labels.Selector) (ret []*Token, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Token))
	})
	return
}

func (l *tokenLister) Get(namespace, name string) (*Token, error) {
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
			Group:    TokenGroupVersionKind.Group,
			Resource: "token",
		}, key)
	}
	return obj.(*Token), nil
}

type tokenController struct {
	controller.GenericController
}

func (c *tokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *tokenController) Lister() TokenLister {
	return &tokenLister{
		controller: c,
	}
}

func (c *tokenController) AddHandler(ctx context.Context, name string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Token); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *tokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Token); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *tokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Token); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *tokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Token); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type tokenFactory struct {
}

func (c tokenFactory) Object() runtime.Object {
	return &Token{}
}

func (c tokenFactory) List() runtime.Object {
	return &TokenList{}
}

func (s *tokenClient) Controller() TokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.tokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &tokenController{
		GenericController: genericController,
	}

	s.client.tokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type tokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TokenController
}

func (s *tokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *tokenClient) Create(o *Token) (*Token, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Token), err
}

func (s *tokenClient) Get(name string, opts metav1.GetOptions) (*Token, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Token), err
}

func (s *tokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Token, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Token), err
}

func (s *tokenClient) Update(o *Token) (*Token, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Token), err
}

func (s *tokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *tokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *tokenClient) List(opts metav1.ListOptions) (*TokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TokenList), err
}

func (s *tokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*TokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*TokenList), err
}

func (s *tokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *tokenClient) Patch(o *Token, patchType types.PatchType, data []byte, subresources ...string) (*Token, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Token), err
}

func (s *tokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *tokenClient) AddHandler(ctx context.Context, name string, sync TokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *tokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *tokenClient) AddLifecycle(ctx context.Context, name string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *tokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *tokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type TokenIndexer func(obj *Token) ([]string, error)

type TokenClientCache interface {
	Get(namespace, name string) (*Token, error)
	List(namespace string, selector labels.Selector) ([]*Token, error)

	Index(name string, indexer TokenIndexer)
	GetIndexed(name, key string) ([]*Token, error)
}

type TokenClient interface {
	Create(*Token) (*Token, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Token, error)
	Update(*Token) (*Token, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*TokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() TokenClientCache

	OnCreate(ctx context.Context, name string, sync TokenChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync TokenChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync TokenChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() TokenInterface
}

type tokenClientCache struct {
	client *tokenClient2
}

type tokenClient2 struct {
	iface      TokenInterface
	controller TokenController
}

func (n *tokenClient2) Interface() TokenInterface {
	return n.iface
}

func (n *tokenClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *tokenClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *tokenClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *tokenClient2) Create(obj *Token) (*Token, error) {
	return n.iface.Create(obj)
}

func (n *tokenClient2) Get(namespace, name string, opts metav1.GetOptions) (*Token, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *tokenClient2) Update(obj *Token) (*Token, error) {
	return n.iface.Update(obj)
}

func (n *tokenClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *tokenClient2) List(namespace string, opts metav1.ListOptions) (*TokenList, error) {
	return n.iface.List(opts)
}

func (n *tokenClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *tokenClientCache) Get(namespace, name string) (*Token, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *tokenClientCache) List(namespace string, selector labels.Selector) ([]*Token, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *tokenClient2) Cache() TokenClientCache {
	n.loadController()
	return &tokenClientCache{
		client: n,
	}
}

func (n *tokenClient2) OnCreate(ctx context.Context, name string, sync TokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &tokenLifecycleDelegate{create: sync})
}

func (n *tokenClient2) OnChange(ctx context.Context, name string, sync TokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &tokenLifecycleDelegate{update: sync})
}

func (n *tokenClient2) OnRemove(ctx context.Context, name string, sync TokenChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &tokenLifecycleDelegate{remove: sync})
}

func (n *tokenClientCache) Index(name string, indexer TokenIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Token); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *tokenClientCache) GetIndexed(name, key string) ([]*Token, error) {
	var result []*Token
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Token); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *tokenClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type tokenLifecycleDelegate struct {
	create TokenChangeHandlerFunc
	update TokenChangeHandlerFunc
	remove TokenChangeHandlerFunc
}

func (n *tokenLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *tokenLifecycleDelegate) Create(obj *Token) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *tokenLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *tokenLifecycleDelegate) Remove(obj *Token) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *tokenLifecycleDelegate) Updated(obj *Token) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
