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
	SourceCodeProviderGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeProvider",
	}
	SourceCodeProviderResource = metav1.APIResource{
		Name:         "sourcecodeproviders",
		SingularName: "sourcecodeprovider",
		Namespaced:   false,
		Kind:         SourceCodeProviderGroupVersionKind.Kind,
	}
)

func NewSourceCodeProvider(namespace, name string, obj SourceCodeProvider) *SourceCodeProvider {
	obj.APIVersion, obj.Kind = SourceCodeProviderGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeProvider
}

type SourceCodeProviderHandlerFunc func(key string, obj *SourceCodeProvider) (runtime.Object, error)

type SourceCodeProviderChangeHandlerFunc func(obj *SourceCodeProvider) (runtime.Object, error)

type SourceCodeProviderLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeProvider, err error)
	Get(namespace, name string) (*SourceCodeProvider, error)
}

type SourceCodeProviderController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeProviderLister
	AddHandler(ctx context.Context, name string, handler SourceCodeProviderHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SourceCodeProviderHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeProviderInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeProvider) (*SourceCodeProvider, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Update(*SourceCodeProvider) (*SourceCodeProvider, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeProviderController
	AddHandler(ctx context.Context, name string, sync SourceCodeProviderHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeProviderLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeProviderHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeProviderLifecycle)
}

type sourceCodeProviderLister struct {
	controller *sourceCodeProviderController
}

func (l *sourceCodeProviderLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeProvider, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeProvider))
	})
	return
}

func (l *sourceCodeProviderLister) Get(namespace, name string) (*SourceCodeProvider, error) {
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
			Group:    SourceCodeProviderGroupVersionKind.Group,
			Resource: "sourceCodeProvider",
		}, key)
	}
	return obj.(*SourceCodeProvider), nil
}

type sourceCodeProviderController struct {
	controller.GenericController
}

func (c *sourceCodeProviderController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeProviderController) Lister() SourceCodeProviderLister {
	return &sourceCodeProviderLister{
		controller: c,
	}
}

func (c *sourceCodeProviderController) AddHandler(ctx context.Context, name string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeProviderFactory struct {
}

func (c sourceCodeProviderFactory) Object() runtime.Object {
	return &SourceCodeProvider{}
}

func (c sourceCodeProviderFactory) List() runtime.Object {
	return &SourceCodeProviderList{}
}

func (s *sourceCodeProviderClient) Controller() SourceCodeProviderController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeProviderControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeProviderGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeProviderController{
		GenericController: genericController,
	}

	s.client.sourceCodeProviderControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeProviderClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeProviderController
}

func (s *sourceCodeProviderClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeProviderClient) Create(o *SourceCodeProvider) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Get(name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Update(o *SourceCodeProvider) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeProviderClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeProviderClient) List(opts metav1.ListOptions) (*SourceCodeProviderList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeProviderList), err
}

func (s *sourceCodeProviderClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeProviderClient) Patch(o *SourceCodeProvider, patchType types.PatchType, data []byte, subresources ...string) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeProviderClient) AddHandler(ctx context.Context, name string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeProviderClient) AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type SourceCodeProviderIndexer func(obj *SourceCodeProvider) ([]string, error)

type SourceCodeProviderClientCache interface {
	Get(namespace, name string) (*SourceCodeProvider, error)
	List(namespace string, selector labels.Selector) ([]*SourceCodeProvider, error)

	Index(name string, indexer SourceCodeProviderIndexer)
	GetIndexed(name, key string) ([]*SourceCodeProvider, error)
}

type SourceCodeProviderClient interface {
	Create(*SourceCodeProvider) (*SourceCodeProvider, error)
	Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Update(*SourceCodeProvider) (*SourceCodeProvider, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*SourceCodeProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() SourceCodeProviderClientCache

	OnCreate(ctx context.Context, name string, sync SourceCodeProviderChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync SourceCodeProviderChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync SourceCodeProviderChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() SourceCodeProviderInterface
}

type sourceCodeProviderClientCache struct {
	client *sourceCodeProviderClient2
}

type sourceCodeProviderClient2 struct {
	iface      SourceCodeProviderInterface
	controller SourceCodeProviderController
}

func (n *sourceCodeProviderClient2) Interface() SourceCodeProviderInterface {
	return n.iface
}

func (n *sourceCodeProviderClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *sourceCodeProviderClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *sourceCodeProviderClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *sourceCodeProviderClient2) Create(obj *SourceCodeProvider) (*SourceCodeProvider, error) {
	return n.iface.Create(obj)
}

func (n *sourceCodeProviderClient2) Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *sourceCodeProviderClient2) Update(obj *SourceCodeProvider) (*SourceCodeProvider, error) {
	return n.iface.Update(obj)
}

func (n *sourceCodeProviderClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *sourceCodeProviderClient2) List(namespace string, opts metav1.ListOptions) (*SourceCodeProviderList, error) {
	return n.iface.List(opts)
}

func (n *sourceCodeProviderClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *sourceCodeProviderClientCache) Get(namespace, name string) (*SourceCodeProvider, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *sourceCodeProviderClientCache) List(namespace string, selector labels.Selector) ([]*SourceCodeProvider, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *sourceCodeProviderClient2) Cache() SourceCodeProviderClientCache {
	n.loadController()
	return &sourceCodeProviderClientCache{
		client: n,
	}
}

func (n *sourceCodeProviderClient2) OnCreate(ctx context.Context, name string, sync SourceCodeProviderChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &sourceCodeProviderLifecycleDelegate{create: sync})
}

func (n *sourceCodeProviderClient2) OnChange(ctx context.Context, name string, sync SourceCodeProviderChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &sourceCodeProviderLifecycleDelegate{update: sync})
}

func (n *sourceCodeProviderClient2) OnRemove(ctx context.Context, name string, sync SourceCodeProviderChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &sourceCodeProviderLifecycleDelegate{remove: sync})
}

func (n *sourceCodeProviderClientCache) Index(name string, indexer SourceCodeProviderIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*SourceCodeProvider); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *sourceCodeProviderClientCache) GetIndexed(name, key string) ([]*SourceCodeProvider, error) {
	var result []*SourceCodeProvider
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*SourceCodeProvider); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *sourceCodeProviderClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type sourceCodeProviderLifecycleDelegate struct {
	create SourceCodeProviderChangeHandlerFunc
	update SourceCodeProviderChangeHandlerFunc
	remove SourceCodeProviderChangeHandlerFunc
}

func (n *sourceCodeProviderLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *sourceCodeProviderLifecycleDelegate) Create(obj *SourceCodeProvider) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *sourceCodeProviderLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *sourceCodeProviderLifecycleDelegate) Remove(obj *SourceCodeProvider) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *sourceCodeProviderLifecycleDelegate) Updated(obj *SourceCodeProvider) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
