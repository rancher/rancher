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
	NodeDriverGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NodeDriver",
	}
	NodeDriverResource = metav1.APIResource{
		Name:         "nodedrivers",
		SingularName: "nodedriver",
		Namespaced:   false,
		Kind:         NodeDriverGroupVersionKind.Kind,
	}

	NodeDriverGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "nodedrivers",
	}
)

func init() {
	resource.Put(NodeDriverGroupVersionResource)
}

// Deprecated: use v3.NodeDriver instead
type NodeDriver = v3.NodeDriver

func NewNodeDriver(namespace, name string, obj v3.NodeDriver) *v3.NodeDriver {
	obj.APIVersion, obj.Kind = NodeDriverGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodeDriverHandlerFunc func(key string, obj *v3.NodeDriver) (runtime.Object, error)

type NodeDriverChangeHandlerFunc func(obj *v3.NodeDriver) (runtime.Object, error)

type NodeDriverLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NodeDriver, err error)
	Get(namespace, name string) (*v3.NodeDriver, error)
}

type NodeDriverController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NodeDriverLister
	AddHandler(ctx context.Context, name string, handler NodeDriverHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeDriverHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NodeDriverHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NodeDriverHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NodeDriverInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NodeDriver) (*v3.NodeDriver, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NodeDriver, error)
	Get(name string, opts metav1.GetOptions) (*v3.NodeDriver, error)
	Update(*v3.NodeDriver) (*v3.NodeDriver, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NodeDriverList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NodeDriverList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeDriverController
	AddHandler(ctx context.Context, name string, sync NodeDriverHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeDriverHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NodeDriverLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeDriverLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeDriverHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeDriverHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeDriverLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeDriverLifecycle)
}

type nodeDriverLister struct {
	ns         string
	controller *nodeDriverController
}

func (l *nodeDriverLister) List(namespace string, selector labels.Selector) (ret []*v3.NodeDriver, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NodeDriver))
	})
	return
}

func (l *nodeDriverLister) Get(namespace, name string) (*v3.NodeDriver, error) {
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
			Group:    NodeDriverGroupVersionKind.Group,
			Resource: NodeDriverGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NodeDriver), nil
}

type nodeDriverController struct {
	ns string
	controller.GenericController
}

func (c *nodeDriverController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodeDriverController) Lister() NodeDriverLister {
	return &nodeDriverLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *nodeDriverController) AddHandler(ctx context.Context, name string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeDriver); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeDriverController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeDriver); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeDriverController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeDriver); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeDriverController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeDriver); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodeDriverFactory struct {
}

func (c nodeDriverFactory) Object() runtime.Object {
	return &v3.NodeDriver{}
}

func (c nodeDriverFactory) List() runtime.Object {
	return &v3.NodeDriverList{}
}

func (s *nodeDriverClient) Controller() NodeDriverController {
	genericController := controller.NewGenericController(s.ns, NodeDriverGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NodeDriverGroupVersionResource, NodeDriverGroupVersionKind.Kind, false))

	return &nodeDriverController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type nodeDriverClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NodeDriverController
}

func (s *nodeDriverClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *nodeDriverClient) Create(o *v3.NodeDriver) (*v3.NodeDriver, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NodeDriver), err
}

func (s *nodeDriverClient) Get(name string, opts metav1.GetOptions) (*v3.NodeDriver, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NodeDriver), err
}

func (s *nodeDriverClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NodeDriver, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NodeDriver), err
}

func (s *nodeDriverClient) Update(o *v3.NodeDriver) (*v3.NodeDriver, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NodeDriver), err
}

func (s *nodeDriverClient) UpdateStatus(o *v3.NodeDriver) (*v3.NodeDriver, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NodeDriver), err
}

func (s *nodeDriverClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeDriverClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeDriverClient) List(opts metav1.ListOptions) (*v3.NodeDriverList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NodeDriverList), err
}

func (s *nodeDriverClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NodeDriverList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NodeDriverList), err
}

func (s *nodeDriverClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeDriverClient) Patch(o *v3.NodeDriver, patchType types.PatchType, data []byte, subresources ...string) (*v3.NodeDriver, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NodeDriver), err
}

func (s *nodeDriverClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeDriverClient) AddHandler(ctx context.Context, name string, sync NodeDriverHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeDriverClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeDriverHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeDriverClient) AddLifecycle(ctx context.Context, name string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeDriverClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeDriverClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeDriverHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeDriverHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
