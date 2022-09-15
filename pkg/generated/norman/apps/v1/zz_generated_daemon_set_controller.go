package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/apps/v1"
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
	DaemonSetGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DaemonSet",
	}
	DaemonSetResource = metav1.APIResource{
		Name:         "daemonsets",
		SingularName: "daemonset",
		Namespaced:   true,

		Kind: DaemonSetGroupVersionKind.Kind,
	}

	DaemonSetGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "daemonsets",
	}
)

func init() {
	resource.Put(DaemonSetGroupVersionResource)
}

// Deprecated: use v1.DaemonSet instead
type DaemonSet = v1.DaemonSet

func NewDaemonSet(namespace, name string, obj v1.DaemonSet) *v1.DaemonSet {
	obj.APIVersion, obj.Kind = DaemonSetGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DaemonSetHandlerFunc func(key string, obj *v1.DaemonSet) (runtime.Object, error)

type DaemonSetChangeHandlerFunc func(obj *v1.DaemonSet) (runtime.Object, error)

type DaemonSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.DaemonSet, err error)
	Get(namespace, name string) (*v1.DaemonSet, error)
}

type DaemonSetController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DaemonSetLister
	AddHandler(ctx context.Context, name string, handler DaemonSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DaemonSetHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DaemonSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler DaemonSetHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type DaemonSetInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.DaemonSet) (*v1.DaemonSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.DaemonSet, error)
	Get(name string, opts metav1.GetOptions) (*v1.DaemonSet, error)
	Update(*v1.DaemonSet) (*v1.DaemonSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.DaemonSetList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.DaemonSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DaemonSetController
	AddHandler(ctx context.Context, name string, sync DaemonSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DaemonSetHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DaemonSetLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DaemonSetLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DaemonSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DaemonSetHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DaemonSetLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DaemonSetLifecycle)
}

type daemonSetLister struct {
	ns         string
	controller *daemonSetController
}

func (l *daemonSetLister) List(namespace string, selector labels.Selector) (ret []*v1.DaemonSet, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.DaemonSet))
	})
	return
}

func (l *daemonSetLister) Get(namespace, name string) (*v1.DaemonSet, error) {
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
			Group:    DaemonSetGroupVersionKind.Group,
			Resource: DaemonSetGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.DaemonSet), nil
}

type daemonSetController struct {
	ns string
	controller.GenericController
}

func (c *daemonSetController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *daemonSetController) Lister() DaemonSetLister {
	return &daemonSetLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *daemonSetController) AddHandler(ctx context.Context, name string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.DaemonSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *daemonSetController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.DaemonSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *daemonSetController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.DaemonSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *daemonSetController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.DaemonSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type daemonSetFactory struct {
}

func (c daemonSetFactory) Object() runtime.Object {
	return &v1.DaemonSet{}
}

func (c daemonSetFactory) List() runtime.Object {
	return &v1.DaemonSetList{}
}

func (s *daemonSetClient) Controller() DaemonSetController {
	genericController := controller.NewGenericController(s.ns, DaemonSetGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(DaemonSetGroupVersionResource, DaemonSetGroupVersionKind.Kind, true))

	return &daemonSetController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type daemonSetClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DaemonSetController
}

func (s *daemonSetClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *daemonSetClient) Create(o *v1.DaemonSet) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) Get(name string, opts metav1.GetOptions) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) Update(o *v1.DaemonSet) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) UpdateStatus(o *v1.DaemonSet) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *daemonSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *daemonSetClient) List(opts metav1.ListOptions) (*v1.DaemonSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.DaemonSetList), err
}

func (s *daemonSetClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.DaemonSetList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.DaemonSetList), err
}

func (s *daemonSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *daemonSetClient) Patch(o *v1.DaemonSet, patchType types.PatchType, data []byte, subresources ...string) (*v1.DaemonSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.DaemonSet), err
}

func (s *daemonSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *daemonSetClient) AddHandler(ctx context.Context, name string, sync DaemonSetHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *daemonSetClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DaemonSetHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *daemonSetClient) AddLifecycle(ctx context.Context, name string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *daemonSetClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *daemonSetClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DaemonSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *daemonSetClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DaemonSetHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *daemonSetClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *daemonSetClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
