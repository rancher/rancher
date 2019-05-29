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
	ClusterRandomizerGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRandomizer",
	}
	ClusterRandomizerResource = metav1.APIResource{
		Name:         "clusterrandomizers",
		SingularName: "clusterrandomizer",
		Namespaced:   false,
		Kind:         ClusterRandomizerGroupVersionKind.Kind,
	}

	ClusterRandomizerGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterrandomizers",
	}
)

func init() {
	resource.Put(ClusterRandomizerGroupVersionResource)
}

func NewClusterRandomizer(namespace, name string, obj ClusterRandomizer) *ClusterRandomizer {
	obj.APIVersion, obj.Kind = ClusterRandomizerGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRandomizerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRandomizer `json:"items"`
}

type ClusterRandomizerHandlerFunc func(key string, obj *ClusterRandomizer) (runtime.Object, error)

type ClusterRandomizerChangeHandlerFunc func(obj *ClusterRandomizer) (runtime.Object, error)

type ClusterRandomizerLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterRandomizer, err error)
	Get(namespace, name string) (*ClusterRandomizer, error)
}

type ClusterRandomizerController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRandomizerLister
	AddHandler(ctx context.Context, name string, handler ClusterRandomizerHandlerFunc)
	AddFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name string, sync ClusterRandomizerHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRandomizerHandlerFunc)
	AddClusterScopedFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name, clusterName string, handler ClusterRandomizerHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRandomizerInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterRandomizer) (*ClusterRandomizer, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRandomizer, error)
	Get(name string, opts metav1.GetOptions) (*ClusterRandomizer, error)
	Update(*ClusterRandomizer) (*ClusterRandomizer, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRandomizerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRandomizerController
	AddHandler(ctx context.Context, name string, sync ClusterRandomizerHandlerFunc)
	AddFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name string, sync ClusterRandomizerHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRandomizerLifecycle)
	AddFeatureLifecycle(enabled func(string) bool, feat string, ctx context.Context, name string, lifecycle ClusterRandomizerLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRandomizerHandlerFunc)
	AddClusterScopedFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name, clusterName string, sync ClusterRandomizerHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRandomizerLifecycle)
	AddClusterScopedFeatureLifecycle(enabled func(string) bool, feat string, ctx context.Context, name, clusterName string, lifecycle ClusterRandomizerLifecycle)
}

type clusterRandomizerLister struct {
	controller *clusterRandomizerController
}

func (l *clusterRandomizerLister) List(namespace string, selector labels.Selector) (ret []*ClusterRandomizer, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterRandomizer))
	})
	return
}

func (l *clusterRandomizerLister) Get(namespace, name string) (*ClusterRandomizer, error) {
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
			Group:    ClusterRandomizerGroupVersionKind.Group,
			Resource: "clusterRandomizer",
		}, key)
	}
	return obj.(*ClusterRandomizer), nil
}

type clusterRandomizerController struct {
	controller.GenericController
}

func (c *clusterRandomizerController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRandomizerController) Lister() ClusterRandomizerLister {
	return &clusterRandomizerLister{
		controller: c,
	}
}

func (c *clusterRandomizerController) AddHandler(ctx context.Context, name string, handler ClusterRandomizerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRandomizer); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRandomizerController) AddFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name string, handler ClusterRandomizerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled(feat) {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRandomizer); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRandomizerController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRandomizerHandlerFunc) {
	resource.PutClusterScoped(ClusterRandomizerGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRandomizer); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRandomizerController) AddClusterScopedFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name, cluster string, handler ClusterRandomizerHandlerFunc) {
	resource.PutClusterScoped(ClusterRandomizerGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled(feat) {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRandomizer); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRandomizerFactory struct {
}

func (c clusterRandomizerFactory) Object() runtime.Object {
	return &ClusterRandomizer{}
}

func (c clusterRandomizerFactory) List() runtime.Object {
	return &ClusterRandomizerList{}
}

func (s *clusterRandomizerClient) Controller() ClusterRandomizerController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRandomizerControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRandomizerGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRandomizerController{
		GenericController: genericController,
	}

	s.client.clusterRandomizerControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRandomizerClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRandomizerController
}

func (s *clusterRandomizerClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRandomizerClient) Create(o *ClusterRandomizer) (*ClusterRandomizer, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterRandomizer), err
}

func (s *clusterRandomizerClient) Get(name string, opts metav1.GetOptions) (*ClusterRandomizer, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterRandomizer), err
}

func (s *clusterRandomizerClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRandomizer, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterRandomizer), err
}

func (s *clusterRandomizerClient) Update(o *ClusterRandomizer) (*ClusterRandomizer, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterRandomizer), err
}

func (s *clusterRandomizerClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRandomizerClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRandomizerClient) List(opts metav1.ListOptions) (*ClusterRandomizerList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRandomizerList), err
}

func (s *clusterRandomizerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRandomizerClient) Patch(o *ClusterRandomizer, patchType types.PatchType, data []byte, subresources ...string) (*ClusterRandomizer, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterRandomizer), err
}

func (s *clusterRandomizerClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRandomizerClient) AddHandler(ctx context.Context, name string, sync ClusterRandomizerHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRandomizerClient) AddFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name string, sync ClusterRandomizerHandlerFunc) {
	s.Controller().AddFeatureHandler(enabled, feat, ctx, name, sync)
}

func (s *clusterRandomizerClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRandomizerLifecycle) {
	sync := NewClusterRandomizerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRandomizerClient) AddFeatureLifecycle(enabled func(string) bool, feat string, ctx context.Context, name string, lifecycle ClusterRandomizerLifecycle) {
	sync := NewClusterRandomizerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(enabled, feat, ctx, name, sync)
}

func (s *clusterRandomizerClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRandomizerHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRandomizerClient) AddClusterScopedFeatureHandler(enabled func(string) bool, feat string, ctx context.Context, name, clusterName string, sync ClusterRandomizerHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(enabled, feat, ctx, name, clusterName, sync)
}

func (s *clusterRandomizerClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRandomizerLifecycle) {
	sync := NewClusterRandomizerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRandomizerClient) AddClusterScopedFeatureLifecycle(enabled func(string) bool, feat string, ctx context.Context, name, clusterName string, lifecycle ClusterRandomizerLifecycle) {
	sync := NewClusterRandomizerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(enabled, feat, ctx, name, clusterName, sync)
}

type ClusterRandomizerIndexer func(obj *ClusterRandomizer) ([]string, error)

type ClusterRandomizerClientCache interface {
	Get(namespace, name string) (*ClusterRandomizer, error)
	List(namespace string, selector labels.Selector) ([]*ClusterRandomizer, error)

	Index(name string, indexer ClusterRandomizerIndexer)
	GetIndexed(name, key string) ([]*ClusterRandomizer, error)
}

type ClusterRandomizerClient interface {
	Create(*ClusterRandomizer) (*ClusterRandomizer, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ClusterRandomizer, error)
	Update(*ClusterRandomizer) (*ClusterRandomizer, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterRandomizerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterRandomizerClientCache

	OnCreate(ctx context.Context, name string, sync ClusterRandomizerChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterRandomizerChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterRandomizerChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterRandomizerInterface
}

type clusterRandomizerClientCache struct {
	client *clusterRandomizerClient2
}

type clusterRandomizerClient2 struct {
	iface      ClusterRandomizerInterface
	controller ClusterRandomizerController
}

func (n *clusterRandomizerClient2) Interface() ClusterRandomizerInterface {
	return n.iface
}

func (n *clusterRandomizerClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterRandomizerClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterRandomizerClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterRandomizerClient2) Create(obj *ClusterRandomizer) (*ClusterRandomizer, error) {
	return n.iface.Create(obj)
}

func (n *clusterRandomizerClient2) Get(namespace, name string, opts metav1.GetOptions) (*ClusterRandomizer, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterRandomizerClient2) Update(obj *ClusterRandomizer) (*ClusterRandomizer, error) {
	return n.iface.Update(obj)
}

func (n *clusterRandomizerClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterRandomizerClient2) List(namespace string, opts metav1.ListOptions) (*ClusterRandomizerList, error) {
	return n.iface.List(opts)
}

func (n *clusterRandomizerClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterRandomizerClientCache) Get(namespace, name string) (*ClusterRandomizer, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterRandomizerClientCache) List(namespace string, selector labels.Selector) ([]*ClusterRandomizer, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterRandomizerClient2) Cache() ClusterRandomizerClientCache {
	n.loadController()
	return &clusterRandomizerClientCache{
		client: n,
	}
}

func (n *clusterRandomizerClient2) OnCreate(ctx context.Context, name string, sync ClusterRandomizerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterRandomizerLifecycleDelegate{create: sync})
}

func (n *clusterRandomizerClient2) OnChange(ctx context.Context, name string, sync ClusterRandomizerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterRandomizerLifecycleDelegate{update: sync})
}

func (n *clusterRandomizerClient2) OnRemove(ctx context.Context, name string, sync ClusterRandomizerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterRandomizerLifecycleDelegate{remove: sync})
}

func (n *clusterRandomizerClientCache) Index(name string, indexer ClusterRandomizerIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ClusterRandomizer); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterRandomizerClientCache) GetIndexed(name, key string) ([]*ClusterRandomizer, error) {
	var result []*ClusterRandomizer
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ClusterRandomizer); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterRandomizerClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterRandomizerLifecycleDelegate struct {
	create ClusterRandomizerChangeHandlerFunc
	update ClusterRandomizerChangeHandlerFunc
	remove ClusterRandomizerChangeHandlerFunc
}

func (n *clusterRandomizerLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterRandomizerLifecycleDelegate) Create(obj *ClusterRandomizer) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterRandomizerLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterRandomizerLifecycleDelegate) Remove(obj *ClusterRandomizer) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterRandomizerLifecycleDelegate) Updated(obj *ClusterRandomizer) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
