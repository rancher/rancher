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
	AlertmanagerGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Alertmanager",
	}
	AlertmanagerResource = metav1.APIResource{
		Name:         "alertmanagers",
		SingularName: "alertmanager",
		Namespaced:   true,

		Kind: AlertmanagerGroupVersionKind.Kind,
	}

	AlertmanagerGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "alertmanagers",
	}
)

func init() {
	resource.Put(AlertmanagerGroupVersionResource)
}

// Deprecated: use v1.Alertmanager instead
type Alertmanager = v1.Alertmanager

func NewAlertmanager(namespace, name string, obj v1.Alertmanager) *v1.Alertmanager {
	obj.APIVersion, obj.Kind = AlertmanagerGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AlertmanagerHandlerFunc func(key string, obj *v1.Alertmanager) (runtime.Object, error)

type AlertmanagerChangeHandlerFunc func(obj *v1.Alertmanager) (runtime.Object, error)

type AlertmanagerLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Alertmanager, err error)
	Get(namespace, name string) (*v1.Alertmanager, error)
}

type AlertmanagerController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AlertmanagerLister
	AddHandler(ctx context.Context, name string, handler AlertmanagerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AlertmanagerHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AlertmanagerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler AlertmanagerHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type AlertmanagerInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Alertmanager) (*v1.Alertmanager, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Alertmanager, error)
	Get(name string, opts metav1.GetOptions) (*v1.Alertmanager, error)
	Update(*v1.Alertmanager) (*v1.Alertmanager, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.AlertmanagerList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.AlertmanagerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AlertmanagerController
	AddHandler(ctx context.Context, name string, sync AlertmanagerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AlertmanagerHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AlertmanagerLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AlertmanagerLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AlertmanagerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AlertmanagerHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AlertmanagerLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AlertmanagerLifecycle)
}

type alertmanagerLister struct {
	ns         string
	controller *alertmanagerController
}

func (l *alertmanagerLister) List(namespace string, selector labels.Selector) (ret []*v1.Alertmanager, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Alertmanager))
	})
	return
}

func (l *alertmanagerLister) Get(namespace, name string) (*v1.Alertmanager, error) {
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
			Group:    AlertmanagerGroupVersionKind.Group,
			Resource: AlertmanagerGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Alertmanager), nil
}

type alertmanagerController struct {
	ns string
	controller.GenericController
}

func (c *alertmanagerController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *alertmanagerController) Lister() AlertmanagerLister {
	return &alertmanagerLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *alertmanagerController) AddHandler(ctx context.Context, name string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *alertmanagerController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *alertmanagerController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *alertmanagerController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type alertmanagerFactory struct {
}

func (c alertmanagerFactory) Object() runtime.Object {
	return &v1.Alertmanager{}
}

func (c alertmanagerFactory) List() runtime.Object {
	return &v1.AlertmanagerList{}
}

func (s *alertmanagerClient) Controller() AlertmanagerController {
	genericController := controller.NewGenericController(s.ns, AlertmanagerGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(AlertmanagerGroupVersionResource, AlertmanagerGroupVersionKind.Kind, true))

	return &alertmanagerController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type alertmanagerClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AlertmanagerController
}

func (s *alertmanagerClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *alertmanagerClient) Create(o *v1.Alertmanager) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) Get(name string, opts metav1.GetOptions) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) Update(o *v1.Alertmanager) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) UpdateStatus(o *v1.Alertmanager) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *alertmanagerClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *alertmanagerClient) List(opts metav1.ListOptions) (*v1.AlertmanagerList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.AlertmanagerList), err
}

func (s *alertmanagerClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.AlertmanagerList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.AlertmanagerList), err
}

func (s *alertmanagerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *alertmanagerClient) Patch(o *v1.Alertmanager, patchType types.PatchType, data []byte, subresources ...string) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *alertmanagerClient) AddHandler(ctx context.Context, name string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *alertmanagerClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *alertmanagerClient) AddLifecycle(ctx context.Context, name string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *alertmanagerClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *alertmanagerClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *alertmanagerClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *alertmanagerClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *alertmanagerClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
