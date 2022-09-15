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
	NodePoolGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NodePool",
	}
	NodePoolResource = metav1.APIResource{
		Name:         "nodepools",
		SingularName: "nodepool",
		Namespaced:   true,

		Kind: NodePoolGroupVersionKind.Kind,
	}

	NodePoolGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "nodepools",
	}
)

func init() {
	resource.Put(NodePoolGroupVersionResource)
}

// Deprecated: use v3.NodePool instead
type NodePool = v3.NodePool

func NewNodePool(namespace, name string, obj v3.NodePool) *v3.NodePool {
	obj.APIVersion, obj.Kind = NodePoolGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodePoolHandlerFunc func(key string, obj *v3.NodePool) (runtime.Object, error)

type NodePoolChangeHandlerFunc func(obj *v3.NodePool) (runtime.Object, error)

type NodePoolLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NodePool, err error)
	Get(namespace, name string) (*v3.NodePool, error)
}

type NodePoolController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NodePoolLister
	AddHandler(ctx context.Context, name string, handler NodePoolHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodePoolHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NodePoolHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NodePoolHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NodePoolInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NodePool) (*v3.NodePool, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NodePool, error)
	Get(name string, opts metav1.GetOptions) (*v3.NodePool, error)
	Update(*v3.NodePool) (*v3.NodePool, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NodePoolList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NodePoolList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodePoolController
	AddHandler(ctx context.Context, name string, sync NodePoolHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodePoolHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NodePoolLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodePoolLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodePoolHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodePoolHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodePoolLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodePoolLifecycle)
}

type nodePoolLister struct {
	ns         string
	controller *nodePoolController
}

func (l *nodePoolLister) List(namespace string, selector labels.Selector) (ret []*v3.NodePool, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NodePool))
	})
	return
}

func (l *nodePoolLister) Get(namespace, name string) (*v3.NodePool, error) {
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
			Group:    NodePoolGroupVersionKind.Group,
			Resource: NodePoolGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NodePool), nil
}

type nodePoolController struct {
	ns string
	controller.GenericController
}

func (c *nodePoolController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodePoolController) Lister() NodePoolLister {
	return &nodePoolLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *nodePoolController) AddHandler(ctx context.Context, name string, handler NodePoolHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodePool); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodePoolController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NodePoolHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodePool); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodePoolController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NodePoolHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodePool); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodePoolController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NodePoolHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodePool); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodePoolFactory struct {
}

func (c nodePoolFactory) Object() runtime.Object {
	return &v3.NodePool{}
}

func (c nodePoolFactory) List() runtime.Object {
	return &v3.NodePoolList{}
}

func (s *nodePoolClient) Controller() NodePoolController {
	genericController := controller.NewGenericController(s.ns, NodePoolGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NodePoolGroupVersionResource, NodePoolGroupVersionKind.Kind, true))

	return &nodePoolController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type nodePoolClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NodePoolController
}

func (s *nodePoolClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *nodePoolClient) Create(o *v3.NodePool) (*v3.NodePool, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NodePool), err
}

func (s *nodePoolClient) Get(name string, opts metav1.GetOptions) (*v3.NodePool, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NodePool), err
}

func (s *nodePoolClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NodePool, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NodePool), err
}

func (s *nodePoolClient) Update(o *v3.NodePool) (*v3.NodePool, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NodePool), err
}

func (s *nodePoolClient) UpdateStatus(o *v3.NodePool) (*v3.NodePool, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NodePool), err
}

func (s *nodePoolClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodePoolClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodePoolClient) List(opts metav1.ListOptions) (*v3.NodePoolList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NodePoolList), err
}

func (s *nodePoolClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NodePoolList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NodePoolList), err
}

func (s *nodePoolClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodePoolClient) Patch(o *v3.NodePool, patchType types.PatchType, data []byte, subresources ...string) (*v3.NodePool, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NodePool), err
}

func (s *nodePoolClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodePoolClient) AddHandler(ctx context.Context, name string, sync NodePoolHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodePoolClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodePoolHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodePoolClient) AddLifecycle(ctx context.Context, name string, lifecycle NodePoolLifecycle) {
	sync := NewNodePoolLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodePoolClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodePoolLifecycle) {
	sync := NewNodePoolLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodePoolClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodePoolHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodePoolClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodePoolHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *nodePoolClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodePoolLifecycle) {
	sync := NewNodePoolLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodePoolClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodePoolLifecycle) {
	sync := NewNodePoolLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
