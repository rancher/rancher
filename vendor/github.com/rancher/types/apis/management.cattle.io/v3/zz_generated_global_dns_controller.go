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
	GlobalDNSGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalDNS",
	}
	GlobalDNSResource = metav1.APIResource{
		Name:         "globaldnses",
		SingularName: "globaldns",
		Namespaced:   true,

		Kind: GlobalDNSGroupVersionKind.Kind,
	}
)

func NewGlobalDNS(namespace, name string, obj GlobalDNS) *GlobalDNS {
	obj.APIVersion, obj.Kind = GlobalDNSGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalDNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalDNS
}

type GlobalDNSHandlerFunc func(key string, obj *GlobalDNS) (runtime.Object, error)

type GlobalDNSChangeHandlerFunc func(obj *GlobalDNS) (runtime.Object, error)

type GlobalDNSLister interface {
	List(namespace string, selector labels.Selector) (ret []*GlobalDNS, err error)
	Get(namespace, name string) (*GlobalDNS, error)
}

type GlobalDNSController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalDNSLister
	AddHandler(ctx context.Context, name string, handler GlobalDNSHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalDNSHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GlobalDNSInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*GlobalDNS) (*GlobalDNS, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalDNS, error)
	Get(name string, opts metav1.GetOptions) (*GlobalDNS, error)
	Update(*GlobalDNS) (*GlobalDNS, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GlobalDNSList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalDNSController
	AddHandler(ctx context.Context, name string, sync GlobalDNSHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalDNSLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDNSHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDNSLifecycle)
}

type globalDnsLister struct {
	controller *globalDnsController
}

func (l *globalDnsLister) List(namespace string, selector labels.Selector) (ret []*GlobalDNS, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GlobalDNS))
	})
	return
}

func (l *globalDnsLister) Get(namespace, name string) (*GlobalDNS, error) {
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
			Group:    GlobalDNSGroupVersionKind.Group,
			Resource: "globalDns",
		}, key)
	}
	return obj.(*GlobalDNS), nil
}

type globalDnsController struct {
	controller.GenericController
}

func (c *globalDnsController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalDnsController) Lister() GlobalDNSLister {
	return &globalDnsLister{
		controller: c,
	}
}

func (c *globalDnsController) AddHandler(ctx context.Context, name string, handler GlobalDNSHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalDNS); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalDNSHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalDNS); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalDnsFactory struct {
}

func (c globalDnsFactory) Object() runtime.Object {
	return &GlobalDNS{}
}

func (c globalDnsFactory) List() runtime.Object {
	return &GlobalDNSList{}
}

func (s *globalDnsClient) Controller() GlobalDNSController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.globalDnsControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GlobalDNSGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &globalDnsController{
		GenericController: genericController,
	}

	s.client.globalDnsControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type globalDnsClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalDNSController
}

func (s *globalDnsClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalDnsClient) Create(o *GlobalDNS) (*GlobalDNS, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) Get(name string, opts metav1.GetOptions) (*GlobalDNS, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalDNS, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) Update(o *GlobalDNS) (*GlobalDNS, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalDnsClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalDnsClient) List(opts metav1.ListOptions) (*GlobalDNSList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GlobalDNSList), err
}

func (s *globalDnsClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalDnsClient) Patch(o *GlobalDNS, patchType types.PatchType, data []byte, subresources ...string) (*GlobalDNS, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalDnsClient) AddHandler(ctx context.Context, name string, sync GlobalDNSHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalDNSLifecycle) {
	sync := NewGlobalDNSLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDNSHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDNSLifecycle) {
	sync := NewGlobalDNSLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type GlobalDNSIndexer func(obj *GlobalDNS) ([]string, error)

type GlobalDNSClientCache interface {
	Get(namespace, name string) (*GlobalDNS, error)
	List(namespace string, selector labels.Selector) ([]*GlobalDNS, error)

	Index(name string, indexer GlobalDNSIndexer)
	GetIndexed(name, key string) ([]*GlobalDNS, error)
}

type GlobalDNSClient interface {
	Create(*GlobalDNS) (*GlobalDNS, error)
	Get(namespace, name string, opts metav1.GetOptions) (*GlobalDNS, error)
	Update(*GlobalDNS) (*GlobalDNS, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*GlobalDNSList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() GlobalDNSClientCache

	OnCreate(ctx context.Context, name string, sync GlobalDNSChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync GlobalDNSChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync GlobalDNSChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() GlobalDNSInterface
}

type globalDnsClientCache struct {
	client *globalDnsClient2
}

type globalDnsClient2 struct {
	iface      GlobalDNSInterface
	controller GlobalDNSController
}

func (n *globalDnsClient2) Interface() GlobalDNSInterface {
	return n.iface
}

func (n *globalDnsClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *globalDnsClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *globalDnsClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *globalDnsClient2) Create(obj *GlobalDNS) (*GlobalDNS, error) {
	return n.iface.Create(obj)
}

func (n *globalDnsClient2) Get(namespace, name string, opts metav1.GetOptions) (*GlobalDNS, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *globalDnsClient2) Update(obj *GlobalDNS) (*GlobalDNS, error) {
	return n.iface.Update(obj)
}

func (n *globalDnsClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *globalDnsClient2) List(namespace string, opts metav1.ListOptions) (*GlobalDNSList, error) {
	return n.iface.List(opts)
}

func (n *globalDnsClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *globalDnsClientCache) Get(namespace, name string) (*GlobalDNS, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *globalDnsClientCache) List(namespace string, selector labels.Selector) ([]*GlobalDNS, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *globalDnsClient2) Cache() GlobalDNSClientCache {
	n.loadController()
	return &globalDnsClientCache{
		client: n,
	}
}

func (n *globalDnsClient2) OnCreate(ctx context.Context, name string, sync GlobalDNSChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &globalDnsLifecycleDelegate{create: sync})
}

func (n *globalDnsClient2) OnChange(ctx context.Context, name string, sync GlobalDNSChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &globalDnsLifecycleDelegate{update: sync})
}

func (n *globalDnsClient2) OnRemove(ctx context.Context, name string, sync GlobalDNSChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &globalDnsLifecycleDelegate{remove: sync})
}

func (n *globalDnsClientCache) Index(name string, indexer GlobalDNSIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*GlobalDNS); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *globalDnsClientCache) GetIndexed(name, key string) ([]*GlobalDNS, error) {
	var result []*GlobalDNS
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*GlobalDNS); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *globalDnsClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type globalDnsLifecycleDelegate struct {
	create GlobalDNSChangeHandlerFunc
	update GlobalDNSChangeHandlerFunc
	remove GlobalDNSChangeHandlerFunc
}

func (n *globalDnsLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *globalDnsLifecycleDelegate) Create(obj *GlobalDNS) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *globalDnsLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *globalDnsLifecycleDelegate) Remove(obj *GlobalDNS) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *globalDnsLifecycleDelegate) Updated(obj *GlobalDNS) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
