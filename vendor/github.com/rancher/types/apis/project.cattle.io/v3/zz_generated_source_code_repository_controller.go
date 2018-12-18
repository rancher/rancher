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
	SourceCodeRepositoryGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeRepository",
	}
	SourceCodeRepositoryResource = metav1.APIResource{
		Name:         "sourcecoderepositories",
		SingularName: "sourcecoderepository",
		Namespaced:   true,

		Kind: SourceCodeRepositoryGroupVersionKind.Kind,
	}
)

func NewSourceCodeRepository(namespace, name string, obj SourceCodeRepository) *SourceCodeRepository {
	obj.APIVersion, obj.Kind = SourceCodeRepositoryGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeRepository
}

type SourceCodeRepositoryHandlerFunc func(key string, obj *SourceCodeRepository) (runtime.Object, error)

type SourceCodeRepositoryChangeHandlerFunc func(obj *SourceCodeRepository) (runtime.Object, error)

type SourceCodeRepositoryLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeRepository, err error)
	Get(namespace, name string) (*SourceCodeRepository, error)
}

type SourceCodeRepositoryController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeRepositoryLister
	AddHandler(ctx context.Context, name string, handler SourceCodeRepositoryHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SourceCodeRepositoryHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeRepositoryInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeRepository) (*SourceCodeRepository, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeRepository, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeRepository, error)
	Update(*SourceCodeRepository) (*SourceCodeRepository, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeRepositoryList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeRepositoryController
	AddHandler(ctx context.Context, name string, sync SourceCodeRepositoryHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeRepositoryLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeRepositoryHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeRepositoryLifecycle)
}

type sourceCodeRepositoryLister struct {
	controller *sourceCodeRepositoryController
}

func (l *sourceCodeRepositoryLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeRepository, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeRepository))
	})
	return
}

func (l *sourceCodeRepositoryLister) Get(namespace, name string) (*SourceCodeRepository, error) {
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
			Group:    SourceCodeRepositoryGroupVersionKind.Group,
			Resource: "sourceCodeRepository",
		}, key)
	}
	return obj.(*SourceCodeRepository), nil
}

type sourceCodeRepositoryController struct {
	controller.GenericController
}

func (c *sourceCodeRepositoryController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeRepositoryController) Lister() SourceCodeRepositoryLister {
	return &sourceCodeRepositoryLister{
		controller: c,
	}
}

func (c *sourceCodeRepositoryController) AddHandler(ctx context.Context, name string, handler SourceCodeRepositoryHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeRepository); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeRepositoryController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SourceCodeRepositoryHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeRepository); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeRepositoryFactory struct {
}

func (c sourceCodeRepositoryFactory) Object() runtime.Object {
	return &SourceCodeRepository{}
}

func (c sourceCodeRepositoryFactory) List() runtime.Object {
	return &SourceCodeRepositoryList{}
}

func (s *sourceCodeRepositoryClient) Controller() SourceCodeRepositoryController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeRepositoryControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeRepositoryGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeRepositoryController{
		GenericController: genericController,
	}

	s.client.sourceCodeRepositoryControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeRepositoryClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeRepositoryController
}

func (s *sourceCodeRepositoryClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeRepositoryClient) Create(o *SourceCodeRepository) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) Get(name string, opts metav1.GetOptions) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) Update(o *SourceCodeRepository) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeRepositoryClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeRepositoryClient) List(opts metav1.ListOptions) (*SourceCodeRepositoryList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeRepositoryList), err
}

func (s *sourceCodeRepositoryClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeRepositoryClient) Patch(o *SourceCodeRepository, patchType types.PatchType, data []byte, subresources ...string) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeRepositoryClient) AddHandler(ctx context.Context, name string, sync SourceCodeRepositoryHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeRepositoryClient) AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeRepositoryLifecycle) {
	sync := NewSourceCodeRepositoryLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeRepositoryClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeRepositoryHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeRepositoryClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeRepositoryLifecycle) {
	sync := NewSourceCodeRepositoryLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type SourceCodeRepositoryIndexer func(obj *SourceCodeRepository) ([]string, error)

type SourceCodeRepositoryClientCache interface {
	Get(namespace, name string) (*SourceCodeRepository, error)
	List(namespace string, selector labels.Selector) ([]*SourceCodeRepository, error)

	Index(name string, indexer SourceCodeRepositoryIndexer)
	GetIndexed(name, key string) ([]*SourceCodeRepository, error)
}

type SourceCodeRepositoryClient interface {
	Create(*SourceCodeRepository) (*SourceCodeRepository, error)
	Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeRepository, error)
	Update(*SourceCodeRepository) (*SourceCodeRepository, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*SourceCodeRepositoryList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() SourceCodeRepositoryClientCache

	OnCreate(ctx context.Context, name string, sync SourceCodeRepositoryChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync SourceCodeRepositoryChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync SourceCodeRepositoryChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() SourceCodeRepositoryInterface
}

type sourceCodeRepositoryClientCache struct {
	client *sourceCodeRepositoryClient2
}

type sourceCodeRepositoryClient2 struct {
	iface      SourceCodeRepositoryInterface
	controller SourceCodeRepositoryController
}

func (n *sourceCodeRepositoryClient2) Interface() SourceCodeRepositoryInterface {
	return n.iface
}

func (n *sourceCodeRepositoryClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *sourceCodeRepositoryClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *sourceCodeRepositoryClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *sourceCodeRepositoryClient2) Create(obj *SourceCodeRepository) (*SourceCodeRepository, error) {
	return n.iface.Create(obj)
}

func (n *sourceCodeRepositoryClient2) Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeRepository, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *sourceCodeRepositoryClient2) Update(obj *SourceCodeRepository) (*SourceCodeRepository, error) {
	return n.iface.Update(obj)
}

func (n *sourceCodeRepositoryClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *sourceCodeRepositoryClient2) List(namespace string, opts metav1.ListOptions) (*SourceCodeRepositoryList, error) {
	return n.iface.List(opts)
}

func (n *sourceCodeRepositoryClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *sourceCodeRepositoryClientCache) Get(namespace, name string) (*SourceCodeRepository, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *sourceCodeRepositoryClientCache) List(namespace string, selector labels.Selector) ([]*SourceCodeRepository, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *sourceCodeRepositoryClient2) Cache() SourceCodeRepositoryClientCache {
	n.loadController()
	return &sourceCodeRepositoryClientCache{
		client: n,
	}
}

func (n *sourceCodeRepositoryClient2) OnCreate(ctx context.Context, name string, sync SourceCodeRepositoryChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &sourceCodeRepositoryLifecycleDelegate{create: sync})
}

func (n *sourceCodeRepositoryClient2) OnChange(ctx context.Context, name string, sync SourceCodeRepositoryChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &sourceCodeRepositoryLifecycleDelegate{update: sync})
}

func (n *sourceCodeRepositoryClient2) OnRemove(ctx context.Context, name string, sync SourceCodeRepositoryChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &sourceCodeRepositoryLifecycleDelegate{remove: sync})
}

func (n *sourceCodeRepositoryClientCache) Index(name string, indexer SourceCodeRepositoryIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*SourceCodeRepository); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *sourceCodeRepositoryClientCache) GetIndexed(name, key string) ([]*SourceCodeRepository, error) {
	var result []*SourceCodeRepository
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*SourceCodeRepository); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *sourceCodeRepositoryClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type sourceCodeRepositoryLifecycleDelegate struct {
	create SourceCodeRepositoryChangeHandlerFunc
	update SourceCodeRepositoryChangeHandlerFunc
	remove SourceCodeRepositoryChangeHandlerFunc
}

func (n *sourceCodeRepositoryLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *sourceCodeRepositoryLifecycleDelegate) Create(obj *SourceCodeRepository) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *sourceCodeRepositoryLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *sourceCodeRepositoryLifecycleDelegate) Remove(obj *SourceCodeRepository) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *sourceCodeRepositoryLifecycleDelegate) Updated(obj *SourceCodeRepository) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
