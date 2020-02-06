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
	LdapConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "LdapConfig",
	}
	LdapConfigResource = metav1.APIResource{
		Name:         "ldapconfigs",
		SingularName: "ldapconfig",
		Namespaced:   false,
		Kind:         LdapConfigGroupVersionKind.Kind,
	}

	LdapConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "ldapconfigs",
	}
)

func init() {
	resource.Put(LdapConfigGroupVersionResource)
}

func NewLdapConfig(namespace, name string, obj LdapConfig) *LdapConfig {
	obj.APIVersion, obj.Kind = LdapConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type LdapConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LdapConfig `json:"items"`
}

type LdapConfigHandlerFunc func(key string, obj *LdapConfig) (runtime.Object, error)

type LdapConfigChangeHandlerFunc func(obj *LdapConfig) (runtime.Object, error)

type LdapConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*LdapConfig, err error)
	Get(namespace, name string) (*LdapConfig, error)
}

type LdapConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() LdapConfigLister
	AddHandler(ctx context.Context, name string, handler LdapConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync LdapConfigHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler LdapConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler LdapConfigHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type LdapConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*LdapConfig) (*LdapConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*LdapConfig, error)
	Get(name string, opts metav1.GetOptions) (*LdapConfig, error)
	Update(*LdapConfig) (*LdapConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*LdapConfigList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*LdapConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() LdapConfigController
	AddHandler(ctx context.Context, name string, sync LdapConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync LdapConfigHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle LdapConfigLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle LdapConfigLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync LdapConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync LdapConfigHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle LdapConfigLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle LdapConfigLifecycle)
}

type ldapConfigLister struct {
	controller *ldapConfigController
}

func (l *ldapConfigLister) List(namespace string, selector labels.Selector) (ret []*LdapConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*LdapConfig))
	})
	return
}

func (l *ldapConfigLister) Get(namespace, name string) (*LdapConfig, error) {
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
			Group:    LdapConfigGroupVersionKind.Group,
			Resource: "ldapConfig",
		}, key)
	}
	return obj.(*LdapConfig), nil
}

type ldapConfigController struct {
	controller.GenericController
}

func (c *ldapConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *ldapConfigController) Lister() LdapConfigLister {
	return &ldapConfigLister{
		controller: c,
	}
}

func (c *ldapConfigController) AddHandler(ctx context.Context, name string, handler LdapConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*LdapConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *ldapConfigController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler LdapConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*LdapConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *ldapConfigController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler LdapConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*LdapConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *ldapConfigController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler LdapConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*LdapConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type ldapConfigFactory struct {
}

func (c ldapConfigFactory) Object() runtime.Object {
	return &LdapConfig{}
}

func (c ldapConfigFactory) List() runtime.Object {
	return &LdapConfigList{}
}

func (s *ldapConfigClient) Controller() LdapConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.ldapConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(LdapConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &ldapConfigController{
		GenericController: genericController,
	}

	s.client.ldapConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type ldapConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   LdapConfigController
}

func (s *ldapConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *ldapConfigClient) Create(o *LdapConfig) (*LdapConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) Get(name string, opts metav1.GetOptions) (*LdapConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*LdapConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) Update(o *LdapConfig) (*LdapConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *ldapConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *ldapConfigClient) List(opts metav1.ListOptions) (*LdapConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*LdapConfigList), err
}

func (s *ldapConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*LdapConfigList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*LdapConfigList), err
}

func (s *ldapConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *ldapConfigClient) Patch(o *LdapConfig, patchType types.PatchType, data []byte, subresources ...string) (*LdapConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *ldapConfigClient) AddHandler(ctx context.Context, name string, sync LdapConfigHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *ldapConfigClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync LdapConfigHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *ldapConfigClient) AddLifecycle(ctx context.Context, name string, lifecycle LdapConfigLifecycle) {
	sync := NewLdapConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *ldapConfigClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle LdapConfigLifecycle) {
	sync := NewLdapConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *ldapConfigClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync LdapConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *ldapConfigClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync LdapConfigHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *ldapConfigClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle LdapConfigLifecycle) {
	sync := NewLdapConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *ldapConfigClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle LdapConfigLifecycle) {
	sync := NewLdapConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
