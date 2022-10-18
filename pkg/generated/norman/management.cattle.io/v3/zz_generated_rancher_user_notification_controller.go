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
	RancherUserNotificationGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RancherUserNotification",
	}
	RancherUserNotificationResource = metav1.APIResource{
		Name:         "rancherusernotifications",
		SingularName: "rancherusernotification",
		Namespaced:   false,
		Kind:         RancherUserNotificationGroupVersionKind.Kind,
	}

	RancherUserNotificationGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rancherusernotifications",
	}
)

func init() {
	resource.Put(RancherUserNotificationGroupVersionResource)
}

// Deprecated: use v3.RancherUserNotification instead
type RancherUserNotification = v3.RancherUserNotification

func NewRancherUserNotification(namespace, name string, obj v3.RancherUserNotification) *v3.RancherUserNotification {
	obj.APIVersion, obj.Kind = RancherUserNotificationGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RancherUserNotificationHandlerFunc func(key string, obj *v3.RancherUserNotification) (runtime.Object, error)

type RancherUserNotificationChangeHandlerFunc func(obj *v3.RancherUserNotification) (runtime.Object, error)

type RancherUserNotificationLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.RancherUserNotification, err error)
	Get(namespace, name string) (*v3.RancherUserNotification, error)
}

type RancherUserNotificationController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RancherUserNotificationLister
	AddHandler(ctx context.Context, name string, handler RancherUserNotificationHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RancherUserNotificationHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RancherUserNotificationHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RancherUserNotificationHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type RancherUserNotificationInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.RancherUserNotification) (*v3.RancherUserNotification, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RancherUserNotification, error)
	Get(name string, opts metav1.GetOptions) (*v3.RancherUserNotification, error)
	Update(*v3.RancherUserNotification) (*v3.RancherUserNotification, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.RancherUserNotificationList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RancherUserNotificationList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RancherUserNotificationController
	AddHandler(ctx context.Context, name string, sync RancherUserNotificationHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RancherUserNotificationHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RancherUserNotificationLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RancherUserNotificationLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RancherUserNotificationHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RancherUserNotificationHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RancherUserNotificationLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RancherUserNotificationLifecycle)
}

type rancherUserNotificationLister struct {
	ns         string
	controller *rancherUserNotificationController
}

func (l *rancherUserNotificationLister) List(namespace string, selector labels.Selector) (ret []*v3.RancherUserNotification, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.RancherUserNotification))
	})
	return
}

func (l *rancherUserNotificationLister) Get(namespace, name string) (*v3.RancherUserNotification, error) {
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
			Group:    RancherUserNotificationGroupVersionKind.Group,
			Resource: RancherUserNotificationGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.RancherUserNotification), nil
}

type rancherUserNotificationController struct {
	ns string
	controller.GenericController
}

func (c *rancherUserNotificationController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rancherUserNotificationController) Lister() RancherUserNotificationLister {
	return &rancherUserNotificationLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *rancherUserNotificationController) AddHandler(ctx context.Context, name string, handler RancherUserNotificationHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RancherUserNotification); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rancherUserNotificationController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RancherUserNotificationHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RancherUserNotification); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rancherUserNotificationController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RancherUserNotificationHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RancherUserNotification); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rancherUserNotificationController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RancherUserNotificationHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RancherUserNotification); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rancherUserNotificationFactory struct {
}

func (c rancherUserNotificationFactory) Object() runtime.Object {
	return &v3.RancherUserNotification{}
}

func (c rancherUserNotificationFactory) List() runtime.Object {
	return &v3.RancherUserNotificationList{}
}

func (s *rancherUserNotificationClient) Controller() RancherUserNotificationController {
	genericController := controller.NewGenericController(s.ns, RancherUserNotificationGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(RancherUserNotificationGroupVersionResource, RancherUserNotificationGroupVersionKind.Kind, false))

	return &rancherUserNotificationController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type rancherUserNotificationClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RancherUserNotificationController
}

func (s *rancherUserNotificationClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rancherUserNotificationClient) Create(o *v3.RancherUserNotification) (*v3.RancherUserNotification, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.RancherUserNotification), err
}

func (s *rancherUserNotificationClient) Get(name string, opts metav1.GetOptions) (*v3.RancherUserNotification, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.RancherUserNotification), err
}

func (s *rancherUserNotificationClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RancherUserNotification, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.RancherUserNotification), err
}

func (s *rancherUserNotificationClient) Update(o *v3.RancherUserNotification) (*v3.RancherUserNotification, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.RancherUserNotification), err
}

func (s *rancherUserNotificationClient) UpdateStatus(o *v3.RancherUserNotification) (*v3.RancherUserNotification, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.RancherUserNotification), err
}

func (s *rancherUserNotificationClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rancherUserNotificationClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rancherUserNotificationClient) List(opts metav1.ListOptions) (*v3.RancherUserNotificationList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.RancherUserNotificationList), err
}

func (s *rancherUserNotificationClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RancherUserNotificationList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.RancherUserNotificationList), err
}

func (s *rancherUserNotificationClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rancherUserNotificationClient) Patch(o *v3.RancherUserNotification, patchType types.PatchType, data []byte, subresources ...string) (*v3.RancherUserNotification, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.RancherUserNotification), err
}

func (s *rancherUserNotificationClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rancherUserNotificationClient) AddHandler(ctx context.Context, name string, sync RancherUserNotificationHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rancherUserNotificationClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RancherUserNotificationHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rancherUserNotificationClient) AddLifecycle(ctx context.Context, name string, lifecycle RancherUserNotificationLifecycle) {
	sync := NewRancherUserNotificationLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rancherUserNotificationClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RancherUserNotificationLifecycle) {
	sync := NewRancherUserNotificationLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rancherUserNotificationClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RancherUserNotificationHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rancherUserNotificationClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RancherUserNotificationHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rancherUserNotificationClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RancherUserNotificationLifecycle) {
	sync := NewRancherUserNotificationLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rancherUserNotificationClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RancherUserNotificationLifecycle) {
	sync := NewRancherUserNotificationLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
