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
	PrincipalGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Principal",
	}
	PrincipalResource = metav1.APIResource{
		Name:         "principals",
		SingularName: "principal",
		Namespaced:   false,
		Kind:         PrincipalGroupVersionKind.Kind,
	}
)

func NewPrincipal(namespace, name string, obj Principal) *Principal {
	obj.APIVersion, obj.Kind = PrincipalGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PrincipalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Principal
}

type PrincipalHandlerFunc func(key string, obj *Principal) (runtime.Object, error)

type PrincipalChangeHandlerFunc func(obj *Principal) (runtime.Object, error)

type PrincipalLister interface {
	List(namespace string, selector labels.Selector) (ret []*Principal, err error)
	Get(namespace, name string) (*Principal, error)
}

type PrincipalController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PrincipalLister
	AddHandler(ctx context.Context, name string, handler PrincipalHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PrincipalHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PrincipalInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Principal) (*Principal, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Principal, error)
	Get(name string, opts metav1.GetOptions) (*Principal, error)
	Update(*Principal) (*Principal, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PrincipalList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PrincipalController
	AddHandler(ctx context.Context, name string, sync PrincipalHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PrincipalLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PrincipalHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PrincipalLifecycle)
}

type principalLister struct {
	controller *principalController
}

func (l *principalLister) List(namespace string, selector labels.Selector) (ret []*Principal, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Principal))
	})
	return
}

func (l *principalLister) Get(namespace, name string) (*Principal, error) {
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
			Group:    PrincipalGroupVersionKind.Group,
			Resource: "principal",
		}, key)
	}
	return obj.(*Principal), nil
}

type principalController struct {
	controller.GenericController
}

func (c *principalController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *principalController) Lister() PrincipalLister {
	return &principalLister{
		controller: c,
	}
}

func (c *principalController) AddHandler(ctx context.Context, name string, handler PrincipalHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Principal); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *principalController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PrincipalHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Principal); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type principalFactory struct {
}

func (c principalFactory) Object() runtime.Object {
	return &Principal{}
}

func (c principalFactory) List() runtime.Object {
	return &PrincipalList{}
}

func (s *principalClient) Controller() PrincipalController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.principalControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PrincipalGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &principalController{
		GenericController: genericController,
	}

	s.client.principalControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type principalClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PrincipalController
}

func (s *principalClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *principalClient) Create(o *Principal) (*Principal, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Principal), err
}

func (s *principalClient) Get(name string, opts metav1.GetOptions) (*Principal, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Principal), err
}

func (s *principalClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Principal, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Principal), err
}

func (s *principalClient) Update(o *Principal) (*Principal, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Principal), err
}

func (s *principalClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *principalClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *principalClient) List(opts metav1.ListOptions) (*PrincipalList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PrincipalList), err
}

func (s *principalClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *principalClient) Patch(o *Principal, patchType types.PatchType, data []byte, subresources ...string) (*Principal, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Principal), err
}

func (s *principalClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *principalClient) AddHandler(ctx context.Context, name string, sync PrincipalHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *principalClient) AddLifecycle(ctx context.Context, name string, lifecycle PrincipalLifecycle) {
	sync := NewPrincipalLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *principalClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PrincipalHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *principalClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PrincipalLifecycle) {
	sync := NewPrincipalLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type PrincipalIndexer func(obj *Principal) ([]string, error)

type PrincipalClientCache interface {
	Get(namespace, name string) (*Principal, error)
	List(namespace string, selector labels.Selector) ([]*Principal, error)

	Index(name string, indexer PrincipalIndexer)
	GetIndexed(name, key string) ([]*Principal, error)
}

type PrincipalClient interface {
	Create(*Principal) (*Principal, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Principal, error)
	Update(*Principal) (*Principal, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*PrincipalList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() PrincipalClientCache

	OnCreate(ctx context.Context, name string, sync PrincipalChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync PrincipalChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync PrincipalChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() PrincipalInterface
}

type principalClientCache struct {
	client *principalClient2
}

type principalClient2 struct {
	iface      PrincipalInterface
	controller PrincipalController
}

func (n *principalClient2) Interface() PrincipalInterface {
	return n.iface
}

func (n *principalClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *principalClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *principalClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *principalClient2) Create(obj *Principal) (*Principal, error) {
	return n.iface.Create(obj)
}

func (n *principalClient2) Get(namespace, name string, opts metav1.GetOptions) (*Principal, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *principalClient2) Update(obj *Principal) (*Principal, error) {
	return n.iface.Update(obj)
}

func (n *principalClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *principalClient2) List(namespace string, opts metav1.ListOptions) (*PrincipalList, error) {
	return n.iface.List(opts)
}

func (n *principalClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *principalClientCache) Get(namespace, name string) (*Principal, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *principalClientCache) List(namespace string, selector labels.Selector) ([]*Principal, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *principalClient2) Cache() PrincipalClientCache {
	n.loadController()
	return &principalClientCache{
		client: n,
	}
}

func (n *principalClient2) OnCreate(ctx context.Context, name string, sync PrincipalChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &principalLifecycleDelegate{create: sync})
}

func (n *principalClient2) OnChange(ctx context.Context, name string, sync PrincipalChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &principalLifecycleDelegate{update: sync})
}

func (n *principalClient2) OnRemove(ctx context.Context, name string, sync PrincipalChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &principalLifecycleDelegate{remove: sync})
}

func (n *principalClientCache) Index(name string, indexer PrincipalIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Principal); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *principalClientCache) GetIndexed(name, key string) ([]*Principal, error) {
	var result []*Principal
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Principal); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *principalClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type principalLifecycleDelegate struct {
	create PrincipalChangeHandlerFunc
	update PrincipalChangeHandlerFunc
	remove PrincipalChangeHandlerFunc
}

func (n *principalLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *principalLifecycleDelegate) Create(obj *Principal) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *principalLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *principalLifecycleDelegate) Remove(obj *Principal) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *principalLifecycleDelegate) Updated(obj *Principal) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
