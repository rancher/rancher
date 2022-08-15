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
	NotifierGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Notifier",
	}
	NotifierResource = metav1.APIResource{
		Name:         "notifiers",
		SingularName: "notifier",
		Namespaced:   true,

		Kind: NotifierGroupVersionKind.Kind,
	}

	NotifierGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "notifiers",
	}
)

func init() {
	resource.Put(NotifierGroupVersionResource)
}

// Deprecated: use v3.Notifier instead
type Notifier = v3.Notifier

func NewNotifier(namespace, name string, obj v3.Notifier) *v3.Notifier {
	obj.APIVersion, obj.Kind = NotifierGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NotifierHandlerFunc func(key string, obj *v3.Notifier) (runtime.Object, error)

type NotifierChangeHandlerFunc func(obj *v3.Notifier) (runtime.Object, error)

type NotifierLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Notifier, err error)
	Get(namespace, name string) (*v3.Notifier, error)
}

type NotifierController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NotifierLister
	AddHandler(ctx context.Context, name string, handler NotifierHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NotifierHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NotifierHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NotifierHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NotifierInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Notifier) (*v3.Notifier, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Notifier, error)
	Get(name string, opts metav1.GetOptions) (*v3.Notifier, error)
	Update(*v3.Notifier) (*v3.Notifier, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NotifierList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NotifierList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NotifierController
	AddHandler(ctx context.Context, name string, sync NotifierHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NotifierHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NotifierLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NotifierLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NotifierHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NotifierHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NotifierLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NotifierLifecycle)
}

type notifierLister struct {
	ns         string
	controller *notifierController
}

func (l *notifierLister) List(namespace string, selector labels.Selector) (ret []*v3.Notifier, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Notifier))
	})
	return
}

func (l *notifierLister) Get(namespace, name string) (*v3.Notifier, error) {
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
			Group:    NotifierGroupVersionKind.Group,
			Resource: NotifierGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Notifier), nil
}

type notifierController struct {
	ns string
	controller.GenericController
}

func (c *notifierController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *notifierController) Lister() NotifierLister {
	return &notifierLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *notifierController) AddHandler(ctx context.Context, name string, handler NotifierHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Notifier); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *notifierController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NotifierHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Notifier); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *notifierController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NotifierHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Notifier); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *notifierController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NotifierHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Notifier); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type notifierFactory struct {
}

func (c notifierFactory) Object() runtime.Object {
	return &v3.Notifier{}
}

func (c notifierFactory) List() runtime.Object {
	return &v3.NotifierList{}
}

func (s *notifierClient) Controller() NotifierController {
	genericController := controller.NewGenericController(s.ns, NotifierGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NotifierGroupVersionResource, NotifierGroupVersionKind.Kind, true))

	return &notifierController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type notifierClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NotifierController
}

func (s *notifierClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *notifierClient) Create(o *v3.Notifier) (*v3.Notifier, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Notifier), err
}

func (s *notifierClient) Get(name string, opts metav1.GetOptions) (*v3.Notifier, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Notifier), err
}

func (s *notifierClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Notifier, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Notifier), err
}

func (s *notifierClient) Update(o *v3.Notifier) (*v3.Notifier, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Notifier), err
}

func (s *notifierClient) UpdateStatus(o *v3.Notifier) (*v3.Notifier, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Notifier), err
}

func (s *notifierClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *notifierClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *notifierClient) List(opts metav1.ListOptions) (*v3.NotifierList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NotifierList), err
}

func (s *notifierClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NotifierList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NotifierList), err
}

func (s *notifierClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *notifierClient) Patch(o *v3.Notifier, patchType types.PatchType, data []byte, subresources ...string) (*v3.Notifier, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Notifier), err
}

func (s *notifierClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *notifierClient) AddHandler(ctx context.Context, name string, sync NotifierHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *notifierClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NotifierHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *notifierClient) AddLifecycle(ctx context.Context, name string, lifecycle NotifierLifecycle) {
	sync := NewNotifierLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *notifierClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NotifierLifecycle) {
	sync := NewNotifierLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *notifierClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NotifierHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *notifierClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NotifierHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *notifierClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NotifierLifecycle) {
	sync := NewNotifierLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *notifierClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NotifierLifecycle) {
	sync := NewNotifierLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
