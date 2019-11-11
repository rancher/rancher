package v2beta2

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/autoscaling/v2beta2"
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
	HorizontalPodAutoscalerGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "HorizontalPodAutoscaler",
	}
	HorizontalPodAutoscalerResource = metav1.APIResource{
		Name:         "horizontalpodautoscalers",
		SingularName: "horizontalpodautoscaler",
		Namespaced:   true,

		Kind: HorizontalPodAutoscalerGroupVersionKind.Kind,
	}

	HorizontalPodAutoscalerGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "horizontalpodautoscalers",
	}
)

func init() {
	resource.Put(HorizontalPodAutoscalerGroupVersionResource)
}

func NewHorizontalPodAutoscaler(namespace, name string, obj v2beta2.HorizontalPodAutoscaler) *v2beta2.HorizontalPodAutoscaler {
	obj.APIVersion, obj.Kind = HorizontalPodAutoscalerGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type HorizontalPodAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v2beta2.HorizontalPodAutoscaler `json:"items"`
}

type HorizontalPodAutoscalerHandlerFunc func(key string, obj *v2beta2.HorizontalPodAutoscaler) (runtime.Object, error)

type HorizontalPodAutoscalerChangeHandlerFunc func(obj *v2beta2.HorizontalPodAutoscaler) (runtime.Object, error)

type HorizontalPodAutoscalerLister interface {
	List(namespace string, selector labels.Selector) (ret []*v2beta2.HorizontalPodAutoscaler, err error)
	Get(namespace, name string) (*v2beta2.HorizontalPodAutoscaler, error)
}

type HorizontalPodAutoscalerController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() HorizontalPodAutoscalerLister
	AddHandler(ctx context.Context, name string, handler HorizontalPodAutoscalerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync HorizontalPodAutoscalerHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler HorizontalPodAutoscalerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler HorizontalPodAutoscalerHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type HorizontalPodAutoscalerInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v2beta2.HorizontalPodAutoscaler, error)
	Get(name string, opts metav1.GetOptions) (*v2beta2.HorizontalPodAutoscaler, error)
	Update(*v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*HorizontalPodAutoscalerList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*HorizontalPodAutoscalerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() HorizontalPodAutoscalerController
	AddHandler(ctx context.Context, name string, sync HorizontalPodAutoscalerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync HorizontalPodAutoscalerHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle HorizontalPodAutoscalerLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle HorizontalPodAutoscalerLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync HorizontalPodAutoscalerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync HorizontalPodAutoscalerHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle HorizontalPodAutoscalerLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle HorizontalPodAutoscalerLifecycle)
}

type horizontalPodAutoscalerLister struct {
	controller *horizontalPodAutoscalerController
}

func (l *horizontalPodAutoscalerLister) List(namespace string, selector labels.Selector) (ret []*v2beta2.HorizontalPodAutoscaler, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v2beta2.HorizontalPodAutoscaler))
	})
	return
}

func (l *horizontalPodAutoscalerLister) Get(namespace, name string) (*v2beta2.HorizontalPodAutoscaler, error) {
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
			Group:    HorizontalPodAutoscalerGroupVersionKind.Group,
			Resource: "horizontalPodAutoscaler",
		}, key)
	}
	return obj.(*v2beta2.HorizontalPodAutoscaler), nil
}

type horizontalPodAutoscalerController struct {
	controller.GenericController
}

func (c *horizontalPodAutoscalerController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *horizontalPodAutoscalerController) Lister() HorizontalPodAutoscalerLister {
	return &horizontalPodAutoscalerLister{
		controller: c,
	}
}

func (c *horizontalPodAutoscalerController) AddHandler(ctx context.Context, name string, handler HorizontalPodAutoscalerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v2beta2.HorizontalPodAutoscaler); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *horizontalPodAutoscalerController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler HorizontalPodAutoscalerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v2beta2.HorizontalPodAutoscaler); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *horizontalPodAutoscalerController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler HorizontalPodAutoscalerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v2beta2.HorizontalPodAutoscaler); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *horizontalPodAutoscalerController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler HorizontalPodAutoscalerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v2beta2.HorizontalPodAutoscaler); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type horizontalPodAutoscalerFactory struct {
}

func (c horizontalPodAutoscalerFactory) Object() runtime.Object {
	return &v2beta2.HorizontalPodAutoscaler{}
}

func (c horizontalPodAutoscalerFactory) List() runtime.Object {
	return &HorizontalPodAutoscalerList{}
}

func (s *horizontalPodAutoscalerClient) Controller() HorizontalPodAutoscalerController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.horizontalPodAutoscalerControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(HorizontalPodAutoscalerGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &horizontalPodAutoscalerController{
		GenericController: genericController,
	}

	s.client.horizontalPodAutoscalerControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type horizontalPodAutoscalerClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   HorizontalPodAutoscalerController
}

func (s *horizontalPodAutoscalerClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *horizontalPodAutoscalerClient) Create(o *v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v2beta2.HorizontalPodAutoscaler), err
}

func (s *horizontalPodAutoscalerClient) Get(name string, opts metav1.GetOptions) (*v2beta2.HorizontalPodAutoscaler, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v2beta2.HorizontalPodAutoscaler), err
}

func (s *horizontalPodAutoscalerClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v2beta2.HorizontalPodAutoscaler, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v2beta2.HorizontalPodAutoscaler), err
}

func (s *horizontalPodAutoscalerClient) Update(o *v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v2beta2.HorizontalPodAutoscaler), err
}

func (s *horizontalPodAutoscalerClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *horizontalPodAutoscalerClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *horizontalPodAutoscalerClient) List(opts metav1.ListOptions) (*HorizontalPodAutoscalerList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*HorizontalPodAutoscalerList), err
}

func (s *horizontalPodAutoscalerClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*HorizontalPodAutoscalerList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*HorizontalPodAutoscalerList), err
}

func (s *horizontalPodAutoscalerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *horizontalPodAutoscalerClient) Patch(o *v2beta2.HorizontalPodAutoscaler, patchType types.PatchType, data []byte, subresources ...string) (*v2beta2.HorizontalPodAutoscaler, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v2beta2.HorizontalPodAutoscaler), err
}

func (s *horizontalPodAutoscalerClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *horizontalPodAutoscalerClient) AddHandler(ctx context.Context, name string, sync HorizontalPodAutoscalerHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *horizontalPodAutoscalerClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync HorizontalPodAutoscalerHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *horizontalPodAutoscalerClient) AddLifecycle(ctx context.Context, name string, lifecycle HorizontalPodAutoscalerLifecycle) {
	sync := NewHorizontalPodAutoscalerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *horizontalPodAutoscalerClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle HorizontalPodAutoscalerLifecycle) {
	sync := NewHorizontalPodAutoscalerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *horizontalPodAutoscalerClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync HorizontalPodAutoscalerHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *horizontalPodAutoscalerClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync HorizontalPodAutoscalerHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *horizontalPodAutoscalerClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle HorizontalPodAutoscalerLifecycle) {
	sync := NewHorizontalPodAutoscalerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *horizontalPodAutoscalerClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle HorizontalPodAutoscalerLifecycle) {
	sync := NewHorizontalPodAutoscalerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type HorizontalPodAutoscalerIndexer func(obj *v2beta2.HorizontalPodAutoscaler) ([]string, error)

type HorizontalPodAutoscalerClientCache interface {
	Get(namespace, name string) (*v2beta2.HorizontalPodAutoscaler, error)
	List(namespace string, selector labels.Selector) ([]*v2beta2.HorizontalPodAutoscaler, error)

	Index(name string, indexer HorizontalPodAutoscalerIndexer)
	GetIndexed(name, key string) ([]*v2beta2.HorizontalPodAutoscaler, error)
}

type HorizontalPodAutoscalerClient interface {
	Create(*v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v2beta2.HorizontalPodAutoscaler, error)
	Update(*v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*HorizontalPodAutoscalerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() HorizontalPodAutoscalerClientCache

	OnCreate(ctx context.Context, name string, sync HorizontalPodAutoscalerChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync HorizontalPodAutoscalerChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync HorizontalPodAutoscalerChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() HorizontalPodAutoscalerInterface
}

type horizontalPodAutoscalerClientCache struct {
	client *horizontalPodAutoscalerClient2
}

type horizontalPodAutoscalerClient2 struct {
	iface      HorizontalPodAutoscalerInterface
	controller HorizontalPodAutoscalerController
}

func (n *horizontalPodAutoscalerClient2) Interface() HorizontalPodAutoscalerInterface {
	return n.iface
}

func (n *horizontalPodAutoscalerClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *horizontalPodAutoscalerClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *horizontalPodAutoscalerClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *horizontalPodAutoscalerClient2) Create(obj *v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error) {
	return n.iface.Create(obj)
}

func (n *horizontalPodAutoscalerClient2) Get(namespace, name string, opts metav1.GetOptions) (*v2beta2.HorizontalPodAutoscaler, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *horizontalPodAutoscalerClient2) Update(obj *v2beta2.HorizontalPodAutoscaler) (*v2beta2.HorizontalPodAutoscaler, error) {
	return n.iface.Update(obj)
}

func (n *horizontalPodAutoscalerClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *horizontalPodAutoscalerClient2) List(namespace string, opts metav1.ListOptions) (*HorizontalPodAutoscalerList, error) {
	return n.iface.List(opts)
}

func (n *horizontalPodAutoscalerClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *horizontalPodAutoscalerClientCache) Get(namespace, name string) (*v2beta2.HorizontalPodAutoscaler, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *horizontalPodAutoscalerClientCache) List(namespace string, selector labels.Selector) ([]*v2beta2.HorizontalPodAutoscaler, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *horizontalPodAutoscalerClient2) Cache() HorizontalPodAutoscalerClientCache {
	n.loadController()
	return &horizontalPodAutoscalerClientCache{
		client: n,
	}
}

func (n *horizontalPodAutoscalerClient2) OnCreate(ctx context.Context, name string, sync HorizontalPodAutoscalerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &horizontalPodAutoscalerLifecycleDelegate{create: sync})
}

func (n *horizontalPodAutoscalerClient2) OnChange(ctx context.Context, name string, sync HorizontalPodAutoscalerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &horizontalPodAutoscalerLifecycleDelegate{update: sync})
}

func (n *horizontalPodAutoscalerClient2) OnRemove(ctx context.Context, name string, sync HorizontalPodAutoscalerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &horizontalPodAutoscalerLifecycleDelegate{remove: sync})
}

func (n *horizontalPodAutoscalerClientCache) Index(name string, indexer HorizontalPodAutoscalerIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v2beta2.HorizontalPodAutoscaler); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *horizontalPodAutoscalerClientCache) GetIndexed(name, key string) ([]*v2beta2.HorizontalPodAutoscaler, error) {
	var result []*v2beta2.HorizontalPodAutoscaler
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v2beta2.HorizontalPodAutoscaler); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *horizontalPodAutoscalerClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type horizontalPodAutoscalerLifecycleDelegate struct {
	create HorizontalPodAutoscalerChangeHandlerFunc
	update HorizontalPodAutoscalerChangeHandlerFunc
	remove HorizontalPodAutoscalerChangeHandlerFunc
}

func (n *horizontalPodAutoscalerLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *horizontalPodAutoscalerLifecycleDelegate) Create(obj *v2beta2.HorizontalPodAutoscaler) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *horizontalPodAutoscalerLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *horizontalPodAutoscalerLifecycleDelegate) Remove(obj *v2beta2.HorizontalPodAutoscaler) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *horizontalPodAutoscalerLifecycleDelegate) Updated(obj *v2beta2.HorizontalPodAutoscaler) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
