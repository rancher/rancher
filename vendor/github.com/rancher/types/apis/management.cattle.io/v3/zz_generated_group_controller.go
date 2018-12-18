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
	GroupGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Group",
	}
	GroupResource = metav1.APIResource{
		Name:         "groups",
		SingularName: "group",
		Namespaced:   false,
		Kind:         GroupGroupVersionKind.Kind,
	}
)

func NewGroup(namespace, name string, obj Group) *Group {
	obj.APIVersion, obj.Kind = GroupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group
}

type GroupHandlerFunc func(key string, obj *Group) (runtime.Object, error)

type GroupChangeHandlerFunc func(obj *Group) (runtime.Object, error)

type GroupLister interface {
	List(namespace string, selector labels.Selector) (ret []*Group, err error)
	Get(namespace, name string) (*Group, error)
}

type GroupController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GroupLister
	AddHandler(ctx context.Context, name string, handler GroupHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GroupHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GroupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Group) (*Group, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Group, error)
	Get(name string, opts metav1.GetOptions) (*Group, error)
	Update(*Group) (*Group, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GroupController
	AddHandler(ctx context.Context, name string, sync GroupHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GroupLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GroupHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GroupLifecycle)
}

type groupLister struct {
	controller *groupController
}

func (l *groupLister) List(namespace string, selector labels.Selector) (ret []*Group, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Group))
	})
	return
}

func (l *groupLister) Get(namespace, name string) (*Group, error) {
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
			Group:    GroupGroupVersionKind.Group,
			Resource: "group",
		}, key)
	}
	return obj.(*Group), nil
}

type groupController struct {
	controller.GenericController
}

func (c *groupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *groupController) Lister() GroupLister {
	return &groupLister{
		controller: c,
	}
}

func (c *groupController) AddHandler(ctx context.Context, name string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Group); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *groupController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Group); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type groupFactory struct {
}

func (c groupFactory) Object() runtime.Object {
	return &Group{}
}

func (c groupFactory) List() runtime.Object {
	return &GroupList{}
}

func (s *groupClient) Controller() GroupController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.groupControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GroupGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &groupController{
		GenericController: genericController,
	}

	s.client.groupControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type groupClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GroupController
}

func (s *groupClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *groupClient) Create(o *Group) (*Group, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Group), err
}

func (s *groupClient) Get(name string, opts metav1.GetOptions) (*Group, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Group), err
}

func (s *groupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Group, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Group), err
}

func (s *groupClient) Update(o *Group) (*Group, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Group), err
}

func (s *groupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *groupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *groupClient) List(opts metav1.ListOptions) (*GroupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GroupList), err
}

func (s *groupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *groupClient) Patch(o *Group, patchType types.PatchType, data []byte, subresources ...string) (*Group, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Group), err
}

func (s *groupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *groupClient) AddHandler(ctx context.Context, name string, sync GroupHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *groupClient) AddLifecycle(ctx context.Context, name string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *groupClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GroupHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *groupClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GroupLifecycle) {
	sync := NewGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type GroupIndexer func(obj *Group) ([]string, error)

type GroupClientCache interface {
	Get(namespace, name string) (*Group, error)
	List(namespace string, selector labels.Selector) ([]*Group, error)

	Index(name string, indexer GroupIndexer)
	GetIndexed(name, key string) ([]*Group, error)
}

type GroupClient interface {
	Create(*Group) (*Group, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Group, error)
	Update(*Group) (*Group, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*GroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() GroupClientCache

	OnCreate(ctx context.Context, name string, sync GroupChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync GroupChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync GroupChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() GroupInterface
}

type groupClientCache struct {
	client *groupClient2
}

type groupClient2 struct {
	iface      GroupInterface
	controller GroupController
}

func (n *groupClient2) Interface() GroupInterface {
	return n.iface
}

func (n *groupClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *groupClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *groupClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *groupClient2) Create(obj *Group) (*Group, error) {
	return n.iface.Create(obj)
}

func (n *groupClient2) Get(namespace, name string, opts metav1.GetOptions) (*Group, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *groupClient2) Update(obj *Group) (*Group, error) {
	return n.iface.Update(obj)
}

func (n *groupClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *groupClient2) List(namespace string, opts metav1.ListOptions) (*GroupList, error) {
	return n.iface.List(opts)
}

func (n *groupClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *groupClientCache) Get(namespace, name string) (*Group, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *groupClientCache) List(namespace string, selector labels.Selector) ([]*Group, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *groupClient2) Cache() GroupClientCache {
	n.loadController()
	return &groupClientCache{
		client: n,
	}
}

func (n *groupClient2) OnCreate(ctx context.Context, name string, sync GroupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &groupLifecycleDelegate{create: sync})
}

func (n *groupClient2) OnChange(ctx context.Context, name string, sync GroupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &groupLifecycleDelegate{update: sync})
}

func (n *groupClient2) OnRemove(ctx context.Context, name string, sync GroupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &groupLifecycleDelegate{remove: sync})
}

func (n *groupClientCache) Index(name string, indexer GroupIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Group); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *groupClientCache) GetIndexed(name, key string) ([]*Group, error) {
	var result []*Group
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Group); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *groupClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type groupLifecycleDelegate struct {
	create GroupChangeHandlerFunc
	update GroupChangeHandlerFunc
	remove GroupChangeHandlerFunc
}

func (n *groupLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *groupLifecycleDelegate) Create(obj *Group) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *groupLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *groupLifecycleDelegate) Remove(obj *Group) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *groupLifecycleDelegate) Updated(obj *Group) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
