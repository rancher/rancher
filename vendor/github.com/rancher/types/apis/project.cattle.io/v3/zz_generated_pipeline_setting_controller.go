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
	PipelineSettingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PipelineSetting",
	}
	PipelineSettingResource = metav1.APIResource{
		Name:         "pipelinesettings",
		SingularName: "pipelinesetting",
		Namespaced:   true,

		Kind: PipelineSettingGroupVersionKind.Kind,
	}

	PipelineSettingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "pipelinesettings",
	}
)

func init() {
	resource.Put(PipelineSettingGroupVersionResource)
}

func NewPipelineSetting(namespace, name string, obj PipelineSetting) *PipelineSetting {
	obj.APIVersion, obj.Kind = PipelineSettingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PipelineSettingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineSetting `json:"items"`
}

type PipelineSettingHandlerFunc func(key string, obj *PipelineSetting) (runtime.Object, error)

type PipelineSettingChangeHandlerFunc func(obj *PipelineSetting) (runtime.Object, error)

type PipelineSettingLister interface {
	List(namespace string, selector labels.Selector) (ret []*PipelineSetting, err error)
	Get(namespace, name string) (*PipelineSetting, error)
}

type PipelineSettingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PipelineSettingLister
	AddHandler(ctx context.Context, name string, handler PipelineSettingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineSettingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PipelineSettingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PipelineSettingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PipelineSettingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*PipelineSetting) (*PipelineSetting, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineSetting, error)
	Get(name string, opts metav1.GetOptions) (*PipelineSetting, error)
	Update(*PipelineSetting) (*PipelineSetting, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PipelineSettingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PipelineSettingController
	AddHandler(ctx context.Context, name string, sync PipelineSettingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineSettingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PipelineSettingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineSettingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineSettingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineSettingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineSettingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineSettingLifecycle)
}

type pipelineSettingLister struct {
	controller *pipelineSettingController
}

func (l *pipelineSettingLister) List(namespace string, selector labels.Selector) (ret []*PipelineSetting, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PipelineSetting))
	})
	return
}

func (l *pipelineSettingLister) Get(namespace, name string) (*PipelineSetting, error) {
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
			Group:    PipelineSettingGroupVersionKind.Group,
			Resource: "pipelineSetting",
		}, key)
	}
	return obj.(*PipelineSetting), nil
}

type pipelineSettingController struct {
	controller.GenericController
}

func (c *pipelineSettingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *pipelineSettingController) Lister() PipelineSettingLister {
	return &pipelineSettingLister{
		controller: c,
	}
}

func (c *pipelineSettingController) AddHandler(ctx context.Context, name string, handler PipelineSettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineSetting); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineSettingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PipelineSettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineSetting); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineSettingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PipelineSettingHandlerFunc) {
	resource.PutClusterScoped(PipelineSettingGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineSetting); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineSettingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PipelineSettingHandlerFunc) {
	resource.PutClusterScoped(PipelineSettingGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineSetting); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type pipelineSettingFactory struct {
}

func (c pipelineSettingFactory) Object() runtime.Object {
	return &PipelineSetting{}
}

func (c pipelineSettingFactory) List() runtime.Object {
	return &PipelineSettingList{}
}

func (s *pipelineSettingClient) Controller() PipelineSettingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.pipelineSettingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PipelineSettingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &pipelineSettingController{
		GenericController: genericController,
	}

	s.client.pipelineSettingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type pipelineSettingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PipelineSettingController
}

func (s *pipelineSettingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *pipelineSettingClient) Create(o *PipelineSetting) (*PipelineSetting, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PipelineSetting), err
}

func (s *pipelineSettingClient) Get(name string, opts metav1.GetOptions) (*PipelineSetting, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PipelineSetting), err
}

func (s *pipelineSettingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineSetting, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PipelineSetting), err
}

func (s *pipelineSettingClient) Update(o *PipelineSetting) (*PipelineSetting, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PipelineSetting), err
}

func (s *pipelineSettingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineSettingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineSettingClient) List(opts metav1.ListOptions) (*PipelineSettingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PipelineSettingList), err
}

func (s *pipelineSettingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineSettingClient) Patch(o *PipelineSetting, patchType types.PatchType, data []byte, subresources ...string) (*PipelineSetting, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*PipelineSetting), err
}

func (s *pipelineSettingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *pipelineSettingClient) AddHandler(ctx context.Context, name string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineSettingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineSettingClient) AddLifecycle(ctx context.Context, name string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineSettingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineSettingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineSettingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *pipelineSettingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineSettingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type PipelineSettingIndexer func(obj *PipelineSetting) ([]string, error)

type PipelineSettingClientCache interface {
	Get(namespace, name string) (*PipelineSetting, error)
	List(namespace string, selector labels.Selector) ([]*PipelineSetting, error)

	Index(name string, indexer PipelineSettingIndexer)
	GetIndexed(name, key string) ([]*PipelineSetting, error)
}

type PipelineSettingClient interface {
	Create(*PipelineSetting) (*PipelineSetting, error)
	Get(namespace, name string, opts metav1.GetOptions) (*PipelineSetting, error)
	Update(*PipelineSetting) (*PipelineSetting, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*PipelineSettingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() PipelineSettingClientCache

	OnCreate(ctx context.Context, name string, sync PipelineSettingChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync PipelineSettingChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync PipelineSettingChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() PipelineSettingInterface
}

type pipelineSettingClientCache struct {
	client *pipelineSettingClient2
}

type pipelineSettingClient2 struct {
	iface      PipelineSettingInterface
	controller PipelineSettingController
}

func (n *pipelineSettingClient2) Interface() PipelineSettingInterface {
	return n.iface
}

func (n *pipelineSettingClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *pipelineSettingClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *pipelineSettingClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *pipelineSettingClient2) Create(obj *PipelineSetting) (*PipelineSetting, error) {
	return n.iface.Create(obj)
}

func (n *pipelineSettingClient2) Get(namespace, name string, opts metav1.GetOptions) (*PipelineSetting, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *pipelineSettingClient2) Update(obj *PipelineSetting) (*PipelineSetting, error) {
	return n.iface.Update(obj)
}

func (n *pipelineSettingClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *pipelineSettingClient2) List(namespace string, opts metav1.ListOptions) (*PipelineSettingList, error) {
	return n.iface.List(opts)
}

func (n *pipelineSettingClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *pipelineSettingClientCache) Get(namespace, name string) (*PipelineSetting, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *pipelineSettingClientCache) List(namespace string, selector labels.Selector) ([]*PipelineSetting, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *pipelineSettingClient2) Cache() PipelineSettingClientCache {
	n.loadController()
	return &pipelineSettingClientCache{
		client: n,
	}
}

func (n *pipelineSettingClient2) OnCreate(ctx context.Context, name string, sync PipelineSettingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &pipelineSettingLifecycleDelegate{create: sync})
}

func (n *pipelineSettingClient2) OnChange(ctx context.Context, name string, sync PipelineSettingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &pipelineSettingLifecycleDelegate{update: sync})
}

func (n *pipelineSettingClient2) OnRemove(ctx context.Context, name string, sync PipelineSettingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &pipelineSettingLifecycleDelegate{remove: sync})
}

func (n *pipelineSettingClientCache) Index(name string, indexer PipelineSettingIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*PipelineSetting); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *pipelineSettingClientCache) GetIndexed(name, key string) ([]*PipelineSetting, error) {
	var result []*PipelineSetting
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*PipelineSetting); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *pipelineSettingClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type pipelineSettingLifecycleDelegate struct {
	create PipelineSettingChangeHandlerFunc
	update PipelineSettingChangeHandlerFunc
	remove PipelineSettingChangeHandlerFunc
}

func (n *pipelineSettingLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *pipelineSettingLifecycleDelegate) Create(obj *PipelineSetting) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *pipelineSettingLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *pipelineSettingLifecycleDelegate) Remove(obj *PipelineSetting) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *pipelineSettingLifecycleDelegate) Updated(obj *PipelineSetting) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
