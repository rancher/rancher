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
	StateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "State",
	}
	StateResource = metav1.APIResource{
		Name:         "states",
		SingularName: "state",
		Namespaced:   true,

		Kind: StateGroupVersionKind.Kind,
	}

	StateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "states",
	}
)

func init() {
	resource.Put(StateGroupVersionResource)
}

func NewState(namespace, name string, obj v1.State) *v1.State {
	obj.APIVersion, obj.Kind = StateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type StateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.State `json:"items"`
}

type StateHandlerFunc func(key string, obj *v1.State) (runtime.Object, error)

type StateChangeHandlerFunc func(obj *v1.State) (runtime.Object, error)

type StateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.State, err error)
	Get(namespace, name string) (*v1.State, error)
}

type StateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() StateLister
	AddHandler(ctx context.Context, name string, handler StateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler StateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type StateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.State) (*v1.State, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.State, error)
	Get(name string, opts metav1.GetOptions) (*v1.State, error)
	Update(*v1.State) (*v1.State, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*StateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() StateController
	AddHandler(ctx context.Context, name string, sync StateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle StateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync StateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle StateLifecycle)
}

type stateLister struct {
	controller *stateController
}

func (l *stateLister) List(namespace string, selector labels.Selector) (ret []*v1.State, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.State))
	})
	return
}

func (l *stateLister) Get(namespace, name string) (*v1.State, error) {
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
			Group:    StateGroupVersionKind.Group,
			Resource: "state",
		}, key)
	}
	return obj.(*v1.State), nil
}

type stateController struct {
	controller.GenericController
}

func (c *stateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *stateController) Lister() StateLister {
	return &stateLister{
		controller: c,
	}
}

func (c *stateController) AddHandler(ctx context.Context, name string, handler StateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.State); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *stateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler StateHandlerFunc) {
	resource.PutClusterScoped(StateGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.State); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type stateFactory struct {
}

func (c stateFactory) Object() runtime.Object {
	return &v1.State{}
}

func (c stateFactory) List() runtime.Object {
	return &StateList{}
}

func (s *stateClient) Controller() StateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.stateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(StateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &stateController{
		GenericController: genericController,
	}

	s.client.stateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type stateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   StateController
}

func (s *stateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *stateClient) Create(o *v1.State) (*v1.State, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.State), err
}

func (s *stateClient) Get(name string, opts metav1.GetOptions) (*v1.State, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.State), err
}

func (s *stateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.State, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.State), err
}

func (s *stateClient) Update(o *v1.State) (*v1.State, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.State), err
}

func (s *stateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *stateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *stateClient) List(opts metav1.ListOptions) (*StateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*StateList), err
}

func (s *stateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *stateClient) Patch(o *v1.State, patchType types.PatchType, data []byte, subresources ...string) (*v1.State, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.State), err
}

func (s *stateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *stateClient) AddHandler(ctx context.Context, name string, sync StateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *stateClient) AddLifecycle(ctx context.Context, name string, lifecycle StateLifecycle) {
	sync := NewStateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *stateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync StateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *stateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle StateLifecycle) {
	sync := NewStateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type StateIndexer func(obj *v1.State) ([]string, error)

type StateClientCache interface {
	Get(namespace, name string) (*v1.State, error)
	List(namespace string, selector labels.Selector) ([]*v1.State, error)

	Index(name string, indexer StateIndexer)
	GetIndexed(name, key string) ([]*v1.State, error)
}

type StateClient interface {
	Create(*v1.State) (*v1.State, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.State, error)
	Update(*v1.State) (*v1.State, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*StateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() StateClientCache

	OnCreate(ctx context.Context, name string, sync StateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync StateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync StateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() StateInterface
}

type stateClientCache struct {
	client *stateClient2
}

type stateClient2 struct {
	iface      StateInterface
	controller StateController
}

func (n *stateClient2) Interface() StateInterface {
	return n.iface
}

func (n *stateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *stateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *stateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *stateClient2) Create(obj *v1.State) (*v1.State, error) {
	return n.iface.Create(obj)
}

func (n *stateClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.State, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *stateClient2) Update(obj *v1.State) (*v1.State, error) {
	return n.iface.Update(obj)
}

func (n *stateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *stateClient2) List(namespace string, opts metav1.ListOptions) (*StateList, error) {
	return n.iface.List(opts)
}

func (n *stateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *stateClientCache) Get(namespace, name string) (*v1.State, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *stateClientCache) List(namespace string, selector labels.Selector) ([]*v1.State, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *stateClient2) Cache() StateClientCache {
	n.loadController()
	return &stateClientCache{
		client: n,
	}
}

func (n *stateClient2) OnCreate(ctx context.Context, name string, sync StateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &stateLifecycleDelegate{create: sync})
}

func (n *stateClient2) OnChange(ctx context.Context, name string, sync StateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &stateLifecycleDelegate{update: sync})
}

func (n *stateClient2) OnRemove(ctx context.Context, name string, sync StateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &stateLifecycleDelegate{remove: sync})
}

func (n *stateClientCache) Index(name string, indexer StateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.State); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *stateClientCache) GetIndexed(name, key string) ([]*v1.State, error) {
	var result []*v1.State
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.State); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *stateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type stateLifecycleDelegate struct {
	create StateChangeHandlerFunc
	update StateChangeHandlerFunc
	remove StateChangeHandlerFunc
}

func (n *stateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *stateLifecycleDelegate) Create(obj *v1.State) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *stateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *stateLifecycleDelegate) Remove(obj *v1.State) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *stateLifecycleDelegate) Updated(obj *v1.State) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
