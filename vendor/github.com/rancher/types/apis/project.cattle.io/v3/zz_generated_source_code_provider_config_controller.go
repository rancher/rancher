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
	SourceCodeProviderConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeProviderConfig",
	}
	SourceCodeProviderConfigResource = metav1.APIResource{
		Name:         "sourcecodeproviderconfigs",
		SingularName: "sourcecodeproviderconfig",
		Namespaced:   true,

		Kind: SourceCodeProviderConfigGroupVersionKind.Kind,
	}

	SourceCodeProviderConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "sourcecodeproviderconfigs",
	}
)

func init() {
	resource.Put(SourceCodeProviderConfigGroupVersionResource)
}

func NewSourceCodeProviderConfig(namespace, name string, obj SourceCodeProviderConfig) *SourceCodeProviderConfig {
	obj.APIVersion, obj.Kind = SourceCodeProviderConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeProviderConfig `json:"items"`
}

type SourceCodeProviderConfigHandlerFunc func(key string, obj *SourceCodeProviderConfig) (runtime.Object, error)

type SourceCodeProviderConfigChangeHandlerFunc func(obj *SourceCodeProviderConfig) (runtime.Object, error)

type SourceCodeProviderConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeProviderConfig, err error)
	Get(namespace, name string) (*SourceCodeProviderConfig, error)
}

type SourceCodeProviderConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeProviderConfigLister
	AddHandler(ctx context.Context, name string, handler SourceCodeProviderConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeProviderConfigHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SourceCodeProviderConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler SourceCodeProviderConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeProviderConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error)
	Update(*SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeProviderConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeProviderConfigController
	AddHandler(ctx context.Context, name string, sync SourceCodeProviderConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeProviderConfigHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeProviderConfigLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SourceCodeProviderConfigLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeProviderConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SourceCodeProviderConfigHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeProviderConfigLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SourceCodeProviderConfigLifecycle)
}

type sourceCodeProviderConfigLister struct {
	controller *sourceCodeProviderConfigController
}

func (l *sourceCodeProviderConfigLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeProviderConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeProviderConfig))
	})
	return
}

func (l *sourceCodeProviderConfigLister) Get(namespace, name string) (*SourceCodeProviderConfig, error) {
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
			Group:    SourceCodeProviderConfigGroupVersionKind.Group,
			Resource: "sourceCodeProviderConfig",
		}, key)
	}
	return obj.(*SourceCodeProviderConfig), nil
}

type sourceCodeProviderConfigController struct {
	controller.GenericController
}

func (c *sourceCodeProviderConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeProviderConfigController) Lister() SourceCodeProviderConfigLister {
	return &sourceCodeProviderConfigLister{
		controller: c,
	}
}

func (c *sourceCodeProviderConfigController) AddHandler(ctx context.Context, name string, handler SourceCodeProviderConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProviderConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderConfigController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler SourceCodeProviderConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProviderConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderConfigController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SourceCodeProviderConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProviderConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderConfigController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler SourceCodeProviderConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProviderConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeProviderConfigFactory struct {
}

func (c sourceCodeProviderConfigFactory) Object() runtime.Object {
	return &SourceCodeProviderConfig{}
}

func (c sourceCodeProviderConfigFactory) List() runtime.Object {
	return &SourceCodeProviderConfigList{}
}

func (s *sourceCodeProviderConfigClient) Controller() SourceCodeProviderConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeProviderConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeProviderConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeProviderConfigController{
		GenericController: genericController,
	}

	s.client.sourceCodeProviderConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeProviderConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeProviderConfigController
}

func (s *sourceCodeProviderConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeProviderConfigClient) Create(o *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Get(name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Update(o *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeProviderConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeProviderConfigClient) List(opts metav1.ListOptions) (*SourceCodeProviderConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeProviderConfigList), err
}

func (s *sourceCodeProviderConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeProviderConfigClient) Patch(o *SourceCodeProviderConfig, patchType types.PatchType, data []byte, subresources ...string) (*SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeProviderConfigClient) AddHandler(ctx context.Context, name string, sync SourceCodeProviderConfigHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeProviderConfigClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeProviderConfigHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sourceCodeProviderConfigClient) AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeProviderConfigLifecycle) {
	sync := NewSourceCodeProviderConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeProviderConfigClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SourceCodeProviderConfigLifecycle) {
	sync := NewSourceCodeProviderConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sourceCodeProviderConfigClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeProviderConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeProviderConfigClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SourceCodeProviderConfigHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *sourceCodeProviderConfigClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeProviderConfigLifecycle) {
	sync := NewSourceCodeProviderConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeProviderConfigClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SourceCodeProviderConfigLifecycle) {
	sync := NewSourceCodeProviderConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type SourceCodeProviderConfigIndexer func(obj *SourceCodeProviderConfig) ([]string, error)

type SourceCodeProviderConfigClientCache interface {
	Get(namespace, name string) (*SourceCodeProviderConfig, error)
	List(namespace string, selector labels.Selector) ([]*SourceCodeProviderConfig, error)

	Index(name string, indexer SourceCodeProviderConfigIndexer)
	GetIndexed(name, key string) ([]*SourceCodeProviderConfig, error)
}

type SourceCodeProviderConfigClient interface {
	Create(*SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error)
	Update(*SourceCodeProviderConfig) (*SourceCodeProviderConfig, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*SourceCodeProviderConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() SourceCodeProviderConfigClientCache

	OnCreate(ctx context.Context, name string, sync SourceCodeProviderConfigChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync SourceCodeProviderConfigChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync SourceCodeProviderConfigChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() SourceCodeProviderConfigInterface
}

type sourceCodeProviderConfigClientCache struct {
	client *sourceCodeProviderConfigClient2
}

type sourceCodeProviderConfigClient2 struct {
	iface      SourceCodeProviderConfigInterface
	controller SourceCodeProviderConfigController
}

func (n *sourceCodeProviderConfigClient2) Interface() SourceCodeProviderConfigInterface {
	return n.iface
}

func (n *sourceCodeProviderConfigClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *sourceCodeProviderConfigClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *sourceCodeProviderConfigClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *sourceCodeProviderConfigClient2) Create(obj *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	return n.iface.Create(obj)
}

func (n *sourceCodeProviderConfigClient2) Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeProviderConfig, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *sourceCodeProviderConfigClient2) Update(obj *SourceCodeProviderConfig) (*SourceCodeProviderConfig, error) {
	return n.iface.Update(obj)
}

func (n *sourceCodeProviderConfigClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *sourceCodeProviderConfigClient2) List(namespace string, opts metav1.ListOptions) (*SourceCodeProviderConfigList, error) {
	return n.iface.List(opts)
}

func (n *sourceCodeProviderConfigClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *sourceCodeProviderConfigClientCache) Get(namespace, name string) (*SourceCodeProviderConfig, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *sourceCodeProviderConfigClientCache) List(namespace string, selector labels.Selector) ([]*SourceCodeProviderConfig, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *sourceCodeProviderConfigClient2) Cache() SourceCodeProviderConfigClientCache {
	n.loadController()
	return &sourceCodeProviderConfigClientCache{
		client: n,
	}
}

func (n *sourceCodeProviderConfigClient2) OnCreate(ctx context.Context, name string, sync SourceCodeProviderConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &sourceCodeProviderConfigLifecycleDelegate{create: sync})
}

func (n *sourceCodeProviderConfigClient2) OnChange(ctx context.Context, name string, sync SourceCodeProviderConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &sourceCodeProviderConfigLifecycleDelegate{update: sync})
}

func (n *sourceCodeProviderConfigClient2) OnRemove(ctx context.Context, name string, sync SourceCodeProviderConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &sourceCodeProviderConfigLifecycleDelegate{remove: sync})
}

func (n *sourceCodeProviderConfigClientCache) Index(name string, indexer SourceCodeProviderConfigIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*SourceCodeProviderConfig); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *sourceCodeProviderConfigClientCache) GetIndexed(name, key string) ([]*SourceCodeProviderConfig, error) {
	var result []*SourceCodeProviderConfig
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*SourceCodeProviderConfig); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *sourceCodeProviderConfigClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type sourceCodeProviderConfigLifecycleDelegate struct {
	create SourceCodeProviderConfigChangeHandlerFunc
	update SourceCodeProviderConfigChangeHandlerFunc
	remove SourceCodeProviderConfigChangeHandlerFunc
}

func (n *sourceCodeProviderConfigLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *sourceCodeProviderConfigLifecycleDelegate) Create(obj *SourceCodeProviderConfig) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *sourceCodeProviderConfigLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *sourceCodeProviderConfigLifecycleDelegate) Remove(obj *SourceCodeProviderConfig) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *sourceCodeProviderConfigLifecycleDelegate) Updated(obj *SourceCodeProviderConfig) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
