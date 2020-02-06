package v3

import (
	"context"
	"time"

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
	EnqueueAfter(namespace, name string, after time.Duration)
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
	ListNamespaced(namespace string, opts metav1.ListOptions) (*AuthConfigList, error)
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

func (s *authConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*AuthConfigList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
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
