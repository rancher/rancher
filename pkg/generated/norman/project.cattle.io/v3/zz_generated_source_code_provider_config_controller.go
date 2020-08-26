package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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

// Deprecated use v3.SourceCodeProviderConfig instead
type SourceCodeProviderConfig = v3.SourceCodeProviderConfig

func NewSourceCodeProviderConfig(namespace, name string, obj v3.SourceCodeProviderConfig) *v3.SourceCodeProviderConfig {
	obj.APIVersion, obj.Kind = SourceCodeProviderConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeProviderConfigHandlerFunc func(key string, obj *v3.SourceCodeProviderConfig) (runtime.Object, error)

type SourceCodeProviderConfigChangeHandlerFunc func(obj *v3.SourceCodeProviderConfig) (runtime.Object, error)

type SourceCodeProviderConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.SourceCodeProviderConfig, err error)
	Get(namespace, name string) (*v3.SourceCodeProviderConfig, error)
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
	EnqueueAfter(namespace, name string, after time.Duration)
}

type SourceCodeProviderConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.SourceCodeProviderConfig) (*v3.SourceCodeProviderConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.SourceCodeProviderConfig, error)
	Get(name string, opts metav1.GetOptions) (*v3.SourceCodeProviderConfig, error)
	Update(*v3.SourceCodeProviderConfig) (*v3.SourceCodeProviderConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.SourceCodeProviderConfigList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.SourceCodeProviderConfigList, error)
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

func (l *sourceCodeProviderConfigLister) List(namespace string, selector labels.Selector) (ret []*v3.SourceCodeProviderConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.SourceCodeProviderConfig))
	})
	return
}

func (l *sourceCodeProviderConfigLister) Get(namespace, name string) (*v3.SourceCodeProviderConfig, error) {
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
			Resource: SourceCodeProviderConfigGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.SourceCodeProviderConfig), nil
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
		} else if v, ok := obj.(*v3.SourceCodeProviderConfig); ok {
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
		} else if v, ok := obj.(*v3.SourceCodeProviderConfig); ok {
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
		} else if v, ok := obj.(*v3.SourceCodeProviderConfig); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*v3.SourceCodeProviderConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeProviderConfigFactory struct {
}

func (c sourceCodeProviderConfigFactory) Object() runtime.Object {
	return &v3.SourceCodeProviderConfig{}
}

func (c sourceCodeProviderConfigFactory) List() runtime.Object {
	return &v3.SourceCodeProviderConfigList{}
}

func (s *sourceCodeProviderConfigClient) Controller() SourceCodeProviderConfigController {
	genericController := controller.NewGenericController(s.ns, SourceCodeProviderConfigGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(SourceCodeProviderConfigGroupVersionResource, SourceCodeProviderConfigGroupVersionKind.Kind, true))

	return &sourceCodeProviderConfigController{
		GenericController: genericController,
	}
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

func (s *sourceCodeProviderConfigClient) Create(o *v3.SourceCodeProviderConfig) (*v3.SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Get(name string, opts metav1.GetOptions) (*v3.SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Update(o *v3.SourceCodeProviderConfig) (*v3.SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) UpdateStatus(o *v3.SourceCodeProviderConfig) (*v3.SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.SourceCodeProviderConfig), err
}

func (s *sourceCodeProviderConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeProviderConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeProviderConfigClient) List(opts metav1.ListOptions) (*v3.SourceCodeProviderConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.SourceCodeProviderConfigList), err
}

func (s *sourceCodeProviderConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.SourceCodeProviderConfigList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.SourceCodeProviderConfigList), err
}

func (s *sourceCodeProviderConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeProviderConfigClient) Patch(o *v3.SourceCodeProviderConfig, patchType types.PatchType, data []byte, subresources ...string) (*v3.SourceCodeProviderConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.SourceCodeProviderConfig), err
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
