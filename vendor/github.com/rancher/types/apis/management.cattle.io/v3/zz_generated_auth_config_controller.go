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
	AuthConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "AuthConfig",
	}
	AuthConfigResource = metav1.APIResource{
		Name:         "authconfigs",
		SingularName: "authconfig",
		Namespaced:   false,
		Kind:         AuthConfigGroupVersionKind.Kind,
	}

	AuthConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "authconfigs",
	}
)

func init() {
	resource.Put(AuthConfigGroupVersionResource)
}

func NewAuthConfig(namespace, name string, obj AuthConfig) *AuthConfig {
	obj.APIVersion, obj.Kind = AuthConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AuthConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthConfig `json:"items"`
}

type AuthConfigHandlerFunc func(key string, obj *AuthConfig) (runtime.Object, error)

type AuthConfigChangeHandlerFunc func(obj *AuthConfig) (runtime.Object, error)

type AuthConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*AuthConfig, err error)
	Get(namespace, name string) (*AuthConfig, error)
}

type AuthConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AuthConfigLister
	AddHandler(ctx context.Context, name string, handler AuthConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AuthConfigHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AuthConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler AuthConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type AuthConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*AuthConfig) (*AuthConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*AuthConfig, error)
	Get(name string, opts metav1.GetOptions) (*AuthConfig, error)
	Update(*AuthConfig) (*AuthConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*AuthConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AuthConfigController
	AddHandler(ctx context.Context, name string, sync AuthConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AuthConfigHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AuthConfigLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AuthConfigLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AuthConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AuthConfigHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AuthConfigLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AuthConfigLifecycle)
}

type authConfigLister struct {
	controller *authConfigController
}

func (l *authConfigLister) List(namespace string, selector labels.Selector) (ret []*AuthConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*AuthConfig))
	})
	return
}

func (l *authConfigLister) Get(namespace, name string) (*AuthConfig, error) {
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
			Group:    AuthConfigGroupVersionKind.Group,
			Resource: "authConfig",
		}, key)
	}
	return obj.(*AuthConfig), nil
}

type authConfigController struct {
	controller.GenericController
}

func (c *authConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *authConfigController) Lister() AuthConfigLister {
	return &authConfigLister{
		controller: c,
	}
}

func (c *authConfigController) AddHandler(ctx context.Context, name string, handler AuthConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *authConfigController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler AuthConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *authConfigController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AuthConfigHandlerFunc) {
	resource.PutClusterScoped(AuthConfigGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *authConfigController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler AuthConfigHandlerFunc) {
	resource.PutClusterScoped(AuthConfigGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type authConfigFactory struct {
}

func (c authConfigFactory) Object() runtime.Object {
	return &AuthConfig{}
}

func (c authConfigFactory) List() runtime.Object {
	return &AuthConfigList{}
}

func (s *authConfigClient) Controller() AuthConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.authConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(AuthConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &authConfigController{
		GenericController: genericController,
	}

	s.client.authConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type authConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AuthConfigController
}

func (s *authConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *authConfigClient) Create(o *AuthConfig) (*AuthConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*AuthConfig), err
}

func (s *authConfigClient) Get(name string, opts metav1.GetOptions) (*AuthConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*AuthConfig), err
}

func (s *authConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*AuthConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*AuthConfig), err
}

func (s *authConfigClient) Update(o *AuthConfig) (*AuthConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*AuthConfig), err
}

func (s *authConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *authConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *authConfigClient) List(opts metav1.ListOptions) (*AuthConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*AuthConfigList), err
}

func (s *authConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *authConfigClient) Patch(o *AuthConfig, patchType types.PatchType, data []byte, subresources ...string) (*AuthConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*AuthConfig), err
}

func (s *authConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *authConfigClient) AddHandler(ctx context.Context, name string, sync AuthConfigHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *authConfigClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AuthConfigHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *authConfigClient) AddLifecycle(ctx context.Context, name string, lifecycle AuthConfigLifecycle) {
	sync := NewAuthConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *authConfigClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AuthConfigLifecycle) {
	sync := NewAuthConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *authConfigClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AuthConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *authConfigClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AuthConfigHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *authConfigClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AuthConfigLifecycle) {
	sync := NewAuthConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *authConfigClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AuthConfigLifecycle) {
	sync := NewAuthConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type AuthConfigIndexer func(obj *AuthConfig) ([]string, error)

type AuthConfigClientCache interface {
	Get(namespace, name string) (*AuthConfig, error)
	List(namespace string, selector labels.Selector) ([]*AuthConfig, error)

	Index(name string, indexer AuthConfigIndexer)
	GetIndexed(name, key string) ([]*AuthConfig, error)
}

type AuthConfigClient interface {
	Create(*AuthConfig) (*AuthConfig, error)
	Get(namespace, name string, opts metav1.GetOptions) (*AuthConfig, error)
	Update(*AuthConfig) (*AuthConfig, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*AuthConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() AuthConfigClientCache

	OnCreate(ctx context.Context, name string, sync AuthConfigChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync AuthConfigChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync AuthConfigChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() AuthConfigInterface
}

type authConfigClientCache struct {
	client *authConfigClient2
}

type authConfigClient2 struct {
	iface      AuthConfigInterface
	controller AuthConfigController
}

func (n *authConfigClient2) Interface() AuthConfigInterface {
	return n.iface
}

func (n *authConfigClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *authConfigClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *authConfigClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *authConfigClient2) Create(obj *AuthConfig) (*AuthConfig, error) {
	return n.iface.Create(obj)
}

func (n *authConfigClient2) Get(namespace, name string, opts metav1.GetOptions) (*AuthConfig, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *authConfigClient2) Update(obj *AuthConfig) (*AuthConfig, error) {
	return n.iface.Update(obj)
}

func (n *authConfigClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *authConfigClient2) List(namespace string, opts metav1.ListOptions) (*AuthConfigList, error) {
	return n.iface.List(opts)
}

func (n *authConfigClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *authConfigClientCache) Get(namespace, name string) (*AuthConfig, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *authConfigClientCache) List(namespace string, selector labels.Selector) ([]*AuthConfig, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *authConfigClient2) Cache() AuthConfigClientCache {
	n.loadController()
	return &authConfigClientCache{
		client: n,
	}
}

func (n *authConfigClient2) OnCreate(ctx context.Context, name string, sync AuthConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &authConfigLifecycleDelegate{create: sync})
}

func (n *authConfigClient2) OnChange(ctx context.Context, name string, sync AuthConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &authConfigLifecycleDelegate{update: sync})
}

func (n *authConfigClient2) OnRemove(ctx context.Context, name string, sync AuthConfigChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &authConfigLifecycleDelegate{remove: sync})
}

func (n *authConfigClientCache) Index(name string, indexer AuthConfigIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*AuthConfig); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *authConfigClientCache) GetIndexed(name, key string) ([]*AuthConfig, error) {
	var result []*AuthConfig
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*AuthConfig); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *authConfigClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type authConfigLifecycleDelegate struct {
	create AuthConfigChangeHandlerFunc
	update AuthConfigChangeHandlerFunc
	remove AuthConfigChangeHandlerFunc
}

func (n *authConfigLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *authConfigLifecycleDelegate) Create(obj *AuthConfig) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *authConfigLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *authConfigLifecycleDelegate) Remove(obj *AuthConfig) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *authConfigLifecycleDelegate) Updated(obj *AuthConfig) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
