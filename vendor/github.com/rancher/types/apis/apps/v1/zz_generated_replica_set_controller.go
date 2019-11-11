package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v1 "k8s.io/api/apps/v1"
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
	ReplicaSetGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ReplicaSet",
	}
	ReplicaSetResource = metav1.APIResource{
		Name:         "replicasets",
		SingularName: "replicaset",
		Namespaced:   true,

		Kind: ReplicaSetGroupVersionKind.Kind,
	}

	ReplicaSetGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "replicasets",
	}
)

func init() {
	resource.Put(ReplicaSetGroupVersionResource)
}

func NewReplicaSet(namespace, name string, obj v1.ReplicaSet) *v1.ReplicaSet {
	obj.APIVersion, obj.Kind = ReplicaSetGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ReplicaSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ReplicaSet `json:"items"`
}

type ReplicaSetHandlerFunc func(key string, obj *v1.ReplicaSet) (runtime.Object, error)

type ReplicaSetChangeHandlerFunc func(obj *v1.ReplicaSet) (runtime.Object, error)

type ReplicaSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ReplicaSet, err error)
	Get(namespace, name string) (*v1.ReplicaSet, error)
}

type ReplicaSetController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ReplicaSetLister
	AddHandler(ctx context.Context, name string, handler ReplicaSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicaSetHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ReplicaSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ReplicaSetHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ReplicaSetInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ReplicaSet) (*v1.ReplicaSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicaSet, error)
	Get(name string, opts metav1.GetOptions) (*v1.ReplicaSet, error)
	Update(*v1.ReplicaSet) (*v1.ReplicaSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ReplicaSetList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ReplicaSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ReplicaSetController
	AddHandler(ctx context.Context, name string, sync ReplicaSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicaSetHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ReplicaSetLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ReplicaSetLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ReplicaSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ReplicaSetHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ReplicaSetLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ReplicaSetLifecycle)
}

type replicaSetLister struct {
	controller *replicaSetController
}

func (l *replicaSetLister) List(namespace string, selector labels.Selector) (ret []*v1.ReplicaSet, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ReplicaSet))
	})
	return
}

func (l *replicaSetLister) Get(namespace, name string) (*v1.ReplicaSet, error) {
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
			Group:    ReplicaSetGroupVersionKind.Group,
			Resource: "replicaSet",
		}, key)
	}
	return obj.(*v1.ReplicaSet), nil
}

type replicaSetController struct {
	controller.GenericController
}

func (c *replicaSetController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *replicaSetController) Lister() ReplicaSetLister {
	return &replicaSetLister{
		controller: c,
	}
}

func (c *replicaSetController) AddHandler(ctx context.Context, name string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicaSetController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicaSetController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicaSetController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type replicaSetFactory struct {
}

func (c replicaSetFactory) Object() runtime.Object {
	return &v1.ReplicaSet{}
}

func (c replicaSetFactory) List() runtime.Object {
	return &ReplicaSetList{}
}

func (s *replicaSetClient) Controller() ReplicaSetController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.replicaSetControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ReplicaSetGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &replicaSetController{
		GenericController: genericController,
	}

	s.client.replicaSetControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type replicaSetClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ReplicaSetController
}

func (s *replicaSetClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *replicaSetClient) Create(o *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) Get(name string, opts metav1.GetOptions) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) Update(o *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *replicaSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *replicaSetClient) List(opts metav1.ListOptions) (*ReplicaSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ReplicaSetList), err
}

func (s *replicaSetClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ReplicaSetList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ReplicaSetList), err
}

func (s *replicaSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *replicaSetClient) Patch(o *v1.ReplicaSet, patchType types.PatchType, data []byte, subresources ...string) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *replicaSetClient) AddHandler(ctx context.Context, name string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *replicaSetClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *replicaSetClient) AddLifecycle(ctx context.Context, name string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *replicaSetClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *replicaSetClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ReplicaSetIndexer func(obj *v1.ReplicaSet) ([]string, error)

type ReplicaSetClientCache interface {
	Get(namespace, name string) (*v1.ReplicaSet, error)
	List(namespace string, selector labels.Selector) ([]*v1.ReplicaSet, error)

	Index(name string, indexer ReplicaSetIndexer)
	GetIndexed(name, key string) ([]*v1.ReplicaSet, error)
}

type ReplicaSetClient interface {
	Create(*v1.ReplicaSet) (*v1.ReplicaSet, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.ReplicaSet, error)
	Update(*v1.ReplicaSet) (*v1.ReplicaSet, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ReplicaSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ReplicaSetClientCache

	OnCreate(ctx context.Context, name string, sync ReplicaSetChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ReplicaSetChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ReplicaSetChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ReplicaSetInterface
}

type replicaSetClientCache struct {
	client *replicaSetClient2
}

type replicaSetClient2 struct {
	iface      ReplicaSetInterface
	controller ReplicaSetController
}

func (n *replicaSetClient2) Interface() ReplicaSetInterface {
	return n.iface
}

func (n *replicaSetClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *replicaSetClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *replicaSetClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *replicaSetClient2) Create(obj *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	return n.iface.Create(obj)
}

func (n *replicaSetClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.ReplicaSet, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *replicaSetClient2) Update(obj *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	return n.iface.Update(obj)
}

func (n *replicaSetClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *replicaSetClient2) List(namespace string, opts metav1.ListOptions) (*ReplicaSetList, error) {
	return n.iface.List(opts)
}

func (n *replicaSetClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *replicaSetClientCache) Get(namespace, name string) (*v1.ReplicaSet, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *replicaSetClientCache) List(namespace string, selector labels.Selector) ([]*v1.ReplicaSet, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *replicaSetClient2) Cache() ReplicaSetClientCache {
	n.loadController()
	return &replicaSetClientCache{
		client: n,
	}
}

func (n *replicaSetClient2) OnCreate(ctx context.Context, name string, sync ReplicaSetChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &replicaSetLifecycleDelegate{create: sync})
}

func (n *replicaSetClient2) OnChange(ctx context.Context, name string, sync ReplicaSetChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &replicaSetLifecycleDelegate{update: sync})
}

func (n *replicaSetClient2) OnRemove(ctx context.Context, name string, sync ReplicaSetChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &replicaSetLifecycleDelegate{remove: sync})
}

func (n *replicaSetClientCache) Index(name string, indexer ReplicaSetIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.ReplicaSet); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *replicaSetClientCache) GetIndexed(name, key string) ([]*v1.ReplicaSet, error) {
	var result []*v1.ReplicaSet
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.ReplicaSet); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *replicaSetClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type replicaSetLifecycleDelegate struct {
	create ReplicaSetChangeHandlerFunc
	update ReplicaSetChangeHandlerFunc
	remove ReplicaSetChangeHandlerFunc
}

func (n *replicaSetLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *replicaSetLifecycleDelegate) Create(obj *v1.ReplicaSet) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *replicaSetLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *replicaSetLifecycleDelegate) Remove(obj *v1.ReplicaSet) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *replicaSetLifecycleDelegate) Updated(obj *v1.ReplicaSet) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
