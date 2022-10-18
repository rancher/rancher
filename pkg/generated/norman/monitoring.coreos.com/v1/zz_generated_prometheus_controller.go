package v1

import (
	"context"
	"time"

	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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
	PrometheusGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Prometheus",
	}
	PrometheusResource = metav1.APIResource{
		Name:         "prometheuses",
		SingularName: "prometheus",
		Namespaced:   true,

		Kind: PrometheusGroupVersionKind.Kind,
	}

	PrometheusGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "prometheuses",
	}
)

func init() {
	resource.Put(PrometheusGroupVersionResource)
}

// Deprecated: use v1.Prometheus instead
type Prometheus = v1.Prometheus

func NewPrometheus(namespace, name string, obj v1.Prometheus) *v1.Prometheus {
	obj.APIVersion, obj.Kind = PrometheusGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PrometheusHandlerFunc func(key string, obj *v1.Prometheus) (runtime.Object, error)

type PrometheusChangeHandlerFunc func(obj *v1.Prometheus) (runtime.Object, error)

type PrometheusLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Prometheus, err error)
	Get(namespace, name string) (*v1.Prometheus, error)
}

type PrometheusController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PrometheusLister
	AddHandler(ctx context.Context, name string, handler PrometheusHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PrometheusHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PrometheusHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PrometheusHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PrometheusInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Prometheus) (*v1.Prometheus, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Prometheus, error)
	Get(name string, opts metav1.GetOptions) (*v1.Prometheus, error)
	Update(*v1.Prometheus) (*v1.Prometheus, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.PrometheusList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.PrometheusList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PrometheusController
	AddHandler(ctx context.Context, name string, sync PrometheusHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PrometheusHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PrometheusLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PrometheusLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PrometheusHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PrometheusHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PrometheusLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PrometheusLifecycle)
}

type prometheusLister struct {
	ns         string
	controller *prometheusController
}

func (l *prometheusLister) List(namespace string, selector labels.Selector) (ret []*v1.Prometheus, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Prometheus))
	})
	return
}

func (l *prometheusLister) Get(namespace, name string) (*v1.Prometheus, error) {
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
			Group:    PrometheusGroupVersionKind.Group,
			Resource: PrometheusGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Prometheus), nil
}

type prometheusController struct {
	ns string
	controller.GenericController
}

func (c *prometheusController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *prometheusController) Lister() PrometheusLister {
	return &prometheusLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *prometheusController) AddHandler(ctx context.Context, name string, handler PrometheusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Prometheus); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *prometheusController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PrometheusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Prometheus); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *prometheusController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PrometheusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Prometheus); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *prometheusController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PrometheusHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Prometheus); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type prometheusFactory struct {
}

func (c prometheusFactory) Object() runtime.Object {
	return &v1.Prometheus{}
}

func (c prometheusFactory) List() runtime.Object {
	return &v1.PrometheusList{}
}

func (s *prometheusClient) Controller() PrometheusController {
	genericController := controller.NewGenericController(s.ns, PrometheusGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PrometheusGroupVersionResource, PrometheusGroupVersionKind.Kind, true))

	return &prometheusController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type prometheusClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PrometheusController
}

func (s *prometheusClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *prometheusClient) Create(o *v1.Prometheus) (*v1.Prometheus, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Prometheus), err
}

func (s *prometheusClient) Get(name string, opts metav1.GetOptions) (*v1.Prometheus, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Prometheus), err
}

func (s *prometheusClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Prometheus, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Prometheus), err
}

func (s *prometheusClient) Update(o *v1.Prometheus) (*v1.Prometheus, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Prometheus), err
}

func (s *prometheusClient) UpdateStatus(o *v1.Prometheus) (*v1.Prometheus, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Prometheus), err
}

func (s *prometheusClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *prometheusClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *prometheusClient) List(opts metav1.ListOptions) (*v1.PrometheusList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.PrometheusList), err
}

func (s *prometheusClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.PrometheusList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.PrometheusList), err
}

func (s *prometheusClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *prometheusClient) Patch(o *v1.Prometheus, patchType types.PatchType, data []byte, subresources ...string) (*v1.Prometheus, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Prometheus), err
}

func (s *prometheusClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *prometheusClient) AddHandler(ctx context.Context, name string, sync PrometheusHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *prometheusClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PrometheusHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *prometheusClient) AddLifecycle(ctx context.Context, name string, lifecycle PrometheusLifecycle) {
	sync := NewPrometheusLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *prometheusClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PrometheusLifecycle) {
	sync := NewPrometheusLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *prometheusClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PrometheusHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *prometheusClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PrometheusHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *prometheusClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PrometheusLifecycle) {
	sync := NewPrometheusLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *prometheusClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PrometheusLifecycle) {
	sync := NewPrometheusLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
