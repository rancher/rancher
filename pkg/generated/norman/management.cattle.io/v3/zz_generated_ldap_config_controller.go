package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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

// Deprecated: use v3.LdapConfig instead
type LdapConfig = v3.LdapConfig

func NewLdapConfig(namespace, name string, obj v3.LdapConfig) *v3.LdapConfig {
	obj.APIVersion, obj.Kind = LdapConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type LdapConfigHandlerFunc func(key string, obj *v3.LdapConfig) (runtime.Object, error)

type LdapConfigChangeHandlerFunc func(obj *v3.LdapConfig) (runtime.Object, error)

type LdapConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.LdapConfig, err error)
	Get(namespace, name string) (*v3.LdapConfig, error)
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
}

type LdapConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.LdapConfig) (*v3.LdapConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.LdapConfig, error)
	Get(name string, opts metav1.GetOptions) (*v3.LdapConfig, error)
	Update(*v3.LdapConfig) (*v3.LdapConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.LdapConfigList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.LdapConfigList, error)
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
	ns         string
	controller *ldapConfigController
}

func (l *ldapConfigLister) List(namespace string, selector labels.Selector) (ret []*v3.LdapConfig, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.LdapConfig))
	})
	return
}

func (l *ldapConfigLister) Get(namespace, name string) (*v3.LdapConfig, error) {
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
			Resource: LdapConfigGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.LdapConfig), nil
}

type ldapConfigController struct {
	ns string
	controller.GenericController
}

func (c *ldapConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *ldapConfigController) Lister() LdapConfigLister {
	return &ldapConfigLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *ldapConfigController) AddHandler(ctx context.Context, name string, handler LdapConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.LdapConfig); ok {
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
		} else if v, ok := obj.(*v3.LdapConfig); ok {
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
		} else if v, ok := obj.(*v3.LdapConfig); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*v3.LdapConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type ldapConfigFactory struct {
}

func (c ldapConfigFactory) Object() runtime.Object {
	return &v3.LdapConfig{}
}

func (c ldapConfigFactory) List() runtime.Object {
	return &v3.LdapConfigList{}
}

func (s *ldapConfigClient) Controller() LdapConfigController {
	genericController := controller.NewGenericController(s.ns, LdapConfigGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(LdapConfigGroupVersionResource, LdapConfigGroupVersionKind.Kind, false))

	return &ldapConfigController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *ldapConfigClient) Create(o *v3.LdapConfig) (*v3.LdapConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.LdapConfig), err
}

func (s *ldapConfigClient) Get(name string, opts metav1.GetOptions) (*v3.LdapConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.LdapConfig), err
}

func (s *ldapConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.LdapConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.LdapConfig), err
}

func (s *ldapConfigClient) Update(o *v3.LdapConfig) (*v3.LdapConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.LdapConfig), err
}

func (s *ldapConfigClient) UpdateStatus(o *v3.LdapConfig) (*v3.LdapConfig, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.LdapConfig), err
}

func (s *ldapConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *ldapConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *ldapConfigClient) List(opts metav1.ListOptions) (*v3.LdapConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.LdapConfigList), err
}

func (s *ldapConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.LdapConfigList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.LdapConfigList), err
}

func (s *ldapConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *ldapConfigClient) Patch(o *v3.LdapConfig, patchType types.PatchType, data []byte, subresources ...string) (*v3.LdapConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.LdapConfig), err
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
