package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
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
	ConfigMapGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ConfigMap",
	}
	ConfigMapResource = metav1.APIResource{
		Name:         "configmaps",
		SingularName: "configmap",
		Namespaced:   true,

		Kind: ConfigMapGroupVersionKind.Kind,
	}

	ConfigMapGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "configmaps",
	}
)

func init() {
	resource.Put(ConfigMapGroupVersionResource)
}

// Deprecated: use v1.ConfigMap instead
type ConfigMap = v1.ConfigMap

func NewConfigMap(namespace, name string, obj v1.ConfigMap) *v1.ConfigMap {
	obj.APIVersion, obj.Kind = ConfigMapGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ConfigMapHandlerFunc func(key string, obj *v1.ConfigMap) (runtime.Object, error)

type ConfigMapChangeHandlerFunc func(obj *v1.ConfigMap) (runtime.Object, error)

type ConfigMapLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ConfigMap, err error)
	Get(namespace, name string) (*v1.ConfigMap, error)
}

type ConfigMapController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ConfigMapLister
	AddHandler(ctx context.Context, name string, handler ConfigMapHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ConfigMapHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ConfigMapHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ConfigMapHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ConfigMapInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ConfigMap) (*v1.ConfigMap, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ConfigMap, error)
	Get(name string, opts metav1.GetOptions) (*v1.ConfigMap, error)
	Update(*v1.ConfigMap) (*v1.ConfigMap, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ConfigMapList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ConfigMapList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ConfigMapController
	AddHandler(ctx context.Context, name string, sync ConfigMapHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ConfigMapHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ConfigMapLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ConfigMapLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ConfigMapHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ConfigMapHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ConfigMapLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ConfigMapLifecycle)
}

type configMapLister struct {
	ns         string
	controller *configMapController
}

func (l *configMapLister) List(namespace string, selector labels.Selector) (ret []*v1.ConfigMap, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ConfigMap))
	})
	return
}

func (l *configMapLister) Get(namespace, name string) (*v1.ConfigMap, error) {
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
			Group:    ConfigMapGroupVersionKind.Group,
			Resource: ConfigMapGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ConfigMap), nil
}

type configMapController struct {
	ns string
	controller.GenericController
}

func (c *configMapController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *configMapController) Lister() ConfigMapLister {
	return &configMapLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *configMapController) AddHandler(ctx context.Context, name string, handler ConfigMapHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ConfigMap); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *configMapController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ConfigMapHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ConfigMap); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *configMapController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ConfigMapHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ConfigMap); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *configMapController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ConfigMapHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ConfigMap); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type configMapFactory struct {
}

func (c configMapFactory) Object() runtime.Object {
	return &v1.ConfigMap{}
}

func (c configMapFactory) List() runtime.Object {
	return &v1.ConfigMapList{}
}

func (s *configMapClient) Controller() ConfigMapController {
	genericController := controller.NewGenericController(s.ns, ConfigMapGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ConfigMapGroupVersionResource, ConfigMapGroupVersionKind.Kind, true))

	return &configMapController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type configMapClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ConfigMapController
}

func (s *configMapClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *configMapClient) Create(o *v1.ConfigMap) (*v1.ConfigMap, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ConfigMap), err
}

func (s *configMapClient) Get(name string, opts metav1.GetOptions) (*v1.ConfigMap, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ConfigMap), err
}

func (s *configMapClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ConfigMap, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ConfigMap), err
}

func (s *configMapClient) Update(o *v1.ConfigMap) (*v1.ConfigMap, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ConfigMap), err
}

func (s *configMapClient) UpdateStatus(o *v1.ConfigMap) (*v1.ConfigMap, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ConfigMap), err
}

func (s *configMapClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *configMapClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *configMapClient) List(opts metav1.ListOptions) (*v1.ConfigMapList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ConfigMapList), err
}

func (s *configMapClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ConfigMapList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ConfigMapList), err
}

func (s *configMapClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *configMapClient) Patch(o *v1.ConfigMap, patchType types.PatchType, data []byte, subresources ...string) (*v1.ConfigMap, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ConfigMap), err
}

func (s *configMapClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *configMapClient) AddHandler(ctx context.Context, name string, sync ConfigMapHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *configMapClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ConfigMapHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *configMapClient) AddLifecycle(ctx context.Context, name string, lifecycle ConfigMapLifecycle) {
	sync := NewConfigMapLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *configMapClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ConfigMapLifecycle) {
	sync := NewConfigMapLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *configMapClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ConfigMapHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *configMapClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ConfigMapHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *configMapClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ConfigMapLifecycle) {
	sync := NewConfigMapLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *configMapClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ConfigMapLifecycle) {
	sync := NewConfigMapLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
