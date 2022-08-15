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
	ComposeConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ComposeConfig",
	}
	ComposeConfigResource = metav1.APIResource{
		Name:         "composeconfigs",
		SingularName: "composeconfig",
		Namespaced:   false,
		Kind:         ComposeConfigGroupVersionKind.Kind,
	}

	ComposeConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "composeconfigs",
	}
)

func init() {
	resource.Put(ComposeConfigGroupVersionResource)
}

// Deprecated: use v3.ComposeConfig instead
type ComposeConfig = v3.ComposeConfig

func NewComposeConfig(namespace, name string, obj v3.ComposeConfig) *v3.ComposeConfig {
	obj.APIVersion, obj.Kind = ComposeConfigGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ComposeConfigHandlerFunc func(key string, obj *v3.ComposeConfig) (runtime.Object, error)

type ComposeConfigChangeHandlerFunc func(obj *v3.ComposeConfig) (runtime.Object, error)

type ComposeConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ComposeConfig, err error)
	Get(namespace, name string) (*v3.ComposeConfig, error)
}

type ComposeConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ComposeConfigLister
	AddHandler(ctx context.Context, name string, handler ComposeConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ComposeConfigHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ComposeConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ComposeConfigHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ComposeConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ComposeConfig) (*v3.ComposeConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ComposeConfig, error)
	Get(name string, opts metav1.GetOptions) (*v3.ComposeConfig, error)
	Update(*v3.ComposeConfig) (*v3.ComposeConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ComposeConfigList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ComposeConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ComposeConfigController
	AddHandler(ctx context.Context, name string, sync ComposeConfigHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ComposeConfigHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ComposeConfigLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ComposeConfigLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ComposeConfigHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ComposeConfigHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ComposeConfigLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ComposeConfigLifecycle)
}

type composeConfigLister struct {
	ns         string
	controller *composeConfigController
}

func (l *composeConfigLister) List(namespace string, selector labels.Selector) (ret []*v3.ComposeConfig, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ComposeConfig))
	})
	return
}

func (l *composeConfigLister) Get(namespace, name string) (*v3.ComposeConfig, error) {
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
			Group:    ComposeConfigGroupVersionKind.Group,
			Resource: ComposeConfigGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ComposeConfig), nil
}

type composeConfigController struct {
	ns string
	controller.GenericController
}

func (c *composeConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *composeConfigController) Lister() ComposeConfigLister {
	return &composeConfigLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *composeConfigController) AddHandler(ctx context.Context, name string, handler ComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ComposeConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *composeConfigController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ComposeConfig); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *composeConfigController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ComposeConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *composeConfigController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ComposeConfigHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ComposeConfig); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type composeConfigFactory struct {
}

func (c composeConfigFactory) Object() runtime.Object {
	return &v3.ComposeConfig{}
}

func (c composeConfigFactory) List() runtime.Object {
	return &v3.ComposeConfigList{}
}

func (s *composeConfigClient) Controller() ComposeConfigController {
	genericController := controller.NewGenericController(s.ns, ComposeConfigGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ComposeConfigGroupVersionResource, ComposeConfigGroupVersionKind.Kind, false))

	return &composeConfigController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type composeConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ComposeConfigController
}

func (s *composeConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *composeConfigClient) Create(o *v3.ComposeConfig) (*v3.ComposeConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ComposeConfig), err
}

func (s *composeConfigClient) Get(name string, opts metav1.GetOptions) (*v3.ComposeConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ComposeConfig), err
}

func (s *composeConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ComposeConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ComposeConfig), err
}

func (s *composeConfigClient) Update(o *v3.ComposeConfig) (*v3.ComposeConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ComposeConfig), err
}

func (s *composeConfigClient) UpdateStatus(o *v3.ComposeConfig) (*v3.ComposeConfig, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ComposeConfig), err
}

func (s *composeConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *composeConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *composeConfigClient) List(opts metav1.ListOptions) (*v3.ComposeConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ComposeConfigList), err
}

func (s *composeConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ComposeConfigList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ComposeConfigList), err
}

func (s *composeConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *composeConfigClient) Patch(o *v3.ComposeConfig, patchType types.PatchType, data []byte, subresources ...string) (*v3.ComposeConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ComposeConfig), err
}

func (s *composeConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *composeConfigClient) AddHandler(ctx context.Context, name string, sync ComposeConfigHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *composeConfigClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ComposeConfigHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *composeConfigClient) AddLifecycle(ctx context.Context, name string, lifecycle ComposeConfigLifecycle) {
	sync := NewComposeConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *composeConfigClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ComposeConfigLifecycle) {
	sync := NewComposeConfigLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *composeConfigClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ComposeConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *composeConfigClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ComposeConfigHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *composeConfigClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ComposeConfigLifecycle) {
	sync := NewComposeConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *composeConfigClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ComposeConfigLifecycle) {
	sync := NewComposeConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
