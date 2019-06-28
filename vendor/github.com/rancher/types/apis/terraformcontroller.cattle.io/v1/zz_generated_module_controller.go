package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v1 "github.com/rancher/terraform-controller/pkg/apis/terraformcontroller.cattle.io/v1"
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
	ModuleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Module",
	}
	ModuleResource = metav1.APIResource{
		Name:         "modules",
		SingularName: "module",
		Namespaced:   true,

		Kind: ModuleGroupVersionKind.Kind,
	}

	ModuleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "modules",
	}
)

func init() {
	resource.Put(ModuleGroupVersionResource)
}

func NewModule(namespace, name string, obj v1.Module) *v1.Module {
	obj.APIVersion, obj.Kind = ModuleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Module `json:"items"`
}

type ModuleHandlerFunc func(key string, obj *v1.Module) (runtime.Object, error)

type ModuleChangeHandlerFunc func(obj *v1.Module) (runtime.Object, error)

type ModuleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Module, err error)
	Get(namespace, name string) (*v1.Module, error)
}

type ModuleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ModuleLister
	AddHandler(ctx context.Context, name string, handler ModuleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ModuleHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ModuleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Module) (*v1.Module, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Module, error)
	Get(name string, opts metav1.GetOptions) (*v1.Module, error)
	Update(*v1.Module) (*v1.Module, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ModuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ModuleController
	AddHandler(ctx context.Context, name string, sync ModuleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ModuleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ModuleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ModuleLifecycle)
}

type moduleLister struct {
	controller *moduleController
}

func (l *moduleLister) List(namespace string, selector labels.Selector) (ret []*v1.Module, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Module))
	})
	return
}

func (l *moduleLister) Get(namespace, name string) (*v1.Module, error) {
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
			Group:    ModuleGroupVersionKind.Group,
			Resource: "module",
		}, key)
	}
	return obj.(*v1.Module), nil
}

type moduleController struct {
	controller.GenericController
}

func (c *moduleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *moduleController) Lister() ModuleLister {
	return &moduleLister{
		controller: c,
	}
}

func (c *moduleController) AddHandler(ctx context.Context, name string, handler ModuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Module); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *moduleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ModuleHandlerFunc) {
	resource.PutClusterScoped(ModuleGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Module); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type moduleFactory struct {
}

func (c moduleFactory) Object() runtime.Object {
	return &v1.Module{}
}

func (c moduleFactory) List() runtime.Object {
	return &ModuleList{}
}

func (s *moduleClient) Controller() ModuleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.moduleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ModuleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &moduleController{
		GenericController: genericController,
	}

	s.client.moduleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type moduleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ModuleController
}

func (s *moduleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *moduleClient) Create(o *v1.Module) (*v1.Module, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Module), err
}

func (s *moduleClient) Get(name string, opts metav1.GetOptions) (*v1.Module, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Module), err
}

func (s *moduleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Module, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Module), err
}

func (s *moduleClient) Update(o *v1.Module) (*v1.Module, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Module), err
}

func (s *moduleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *moduleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *moduleClient) List(opts metav1.ListOptions) (*ModuleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ModuleList), err
}

func (s *moduleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *moduleClient) Patch(o *v1.Module, patchType types.PatchType, data []byte, subresources ...string) (*v1.Module, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Module), err
}

func (s *moduleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *moduleClient) AddHandler(ctx context.Context, name string, sync ModuleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *moduleClient) AddLifecycle(ctx context.Context, name string, lifecycle ModuleLifecycle) {
	sync := NewModuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *moduleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ModuleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *moduleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ModuleLifecycle) {
	sync := NewModuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type ModuleIndexer func(obj *v1.Module) ([]string, error)

type ModuleClientCache interface {
	Get(namespace, name string) (*v1.Module, error)
	List(namespace string, selector labels.Selector) ([]*v1.Module, error)

	Index(name string, indexer ModuleIndexer)
	GetIndexed(name, key string) ([]*v1.Module, error)
}

type ModuleClient interface {
	Create(*v1.Module) (*v1.Module, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Module, error)
	Update(*v1.Module) (*v1.Module, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ModuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ModuleClientCache

	OnCreate(ctx context.Context, name string, sync ModuleChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ModuleChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ModuleChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ModuleInterface
}

type moduleClientCache struct {
	client *moduleClient2
}

type moduleClient2 struct {
	iface      ModuleInterface
	controller ModuleController
}

func (n *moduleClient2) Interface() ModuleInterface {
	return n.iface
}

func (n *moduleClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *moduleClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *moduleClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *moduleClient2) Create(obj *v1.Module) (*v1.Module, error) {
	return n.iface.Create(obj)
}

func (n *moduleClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Module, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *moduleClient2) Update(obj *v1.Module) (*v1.Module, error) {
	return n.iface.Update(obj)
}

func (n *moduleClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *moduleClient2) List(namespace string, opts metav1.ListOptions) (*ModuleList, error) {
	return n.iface.List(opts)
}

func (n *moduleClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *moduleClientCache) Get(namespace, name string) (*v1.Module, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *moduleClientCache) List(namespace string, selector labels.Selector) ([]*v1.Module, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *moduleClient2) Cache() ModuleClientCache {
	n.loadController()
	return &moduleClientCache{
		client: n,
	}
}

func (n *moduleClient2) OnCreate(ctx context.Context, name string, sync ModuleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &moduleLifecycleDelegate{create: sync})
}

func (n *moduleClient2) OnChange(ctx context.Context, name string, sync ModuleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &moduleLifecycleDelegate{update: sync})
}

func (n *moduleClient2) OnRemove(ctx context.Context, name string, sync ModuleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &moduleLifecycleDelegate{remove: sync})
}

func (n *moduleClientCache) Index(name string, indexer ModuleIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Module); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *moduleClientCache) GetIndexed(name, key string) ([]*v1.Module, error) {
	var result []*v1.Module
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Module); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *moduleClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type moduleLifecycleDelegate struct {
	create ModuleChangeHandlerFunc
	update ModuleChangeHandlerFunc
	remove ModuleChangeHandlerFunc
}

func (n *moduleLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *moduleLifecycleDelegate) Create(obj *v1.Module) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *moduleLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *moduleLifecycleDelegate) Remove(obj *v1.Module) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *moduleLifecycleDelegate) Updated(obj *v1.Module) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
