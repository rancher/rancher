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
	NodeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Node",
	}
	NodeResource = metav1.APIResource{
		Name:         "nodes",
		SingularName: "node",
		Namespaced:   false,
		Kind:         NodeGroupVersionKind.Kind,
	}

	NodeGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "nodes",
	}
)

func init() {
	resource.Put(NodeGroupVersionResource)
}

// Deprecated: use v1.Node instead
type Node = v1.Node

func NewNode(namespace, name string, obj v1.Node) *v1.Node {
	obj.APIVersion, obj.Kind = NodeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodeHandlerFunc func(key string, obj *v1.Node) (runtime.Object, error)

type NodeChangeHandlerFunc func(obj *v1.Node) (runtime.Object, error)

type NodeLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Node, err error)
	Get(namespace, name string) (*v1.Node, error)
}

type NodeController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NodeLister
	AddHandler(ctx context.Context, name string, handler NodeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NodeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NodeHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NodeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Node) (*v1.Node, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Node, error)
	Get(name string, opts metav1.GetOptions) (*v1.Node, error)
	Update(*v1.Node) (*v1.Node, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.NodeList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.NodeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeController
	AddHandler(ctx context.Context, name string, sync NodeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NodeLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeLifecycle)
}

type nodeLister struct {
	ns         string
	controller *nodeController
}

func (l *nodeLister) List(namespace string, selector labels.Selector) (ret []*v1.Node, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Node))
	})
	return
}

func (l *nodeLister) Get(namespace, name string) (*v1.Node, error) {
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
			Group:    NodeGroupVersionKind.Group,
			Resource: NodeGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Node), nil
}

type nodeController struct {
	ns string
	controller.GenericController
}

func (c *nodeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodeController) Lister() NodeLister {
	return &nodeLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *nodeController) AddHandler(ctx context.Context, name string, handler NodeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Node); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NodeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Node); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NodeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Node); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NodeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Node); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodeFactory struct {
}

func (c nodeFactory) Object() runtime.Object {
	return &v1.Node{}
}

func (c nodeFactory) List() runtime.Object {
	return &v1.NodeList{}
}

func (s *nodeClient) Controller() NodeController {
	genericController := controller.NewGenericController(s.ns, NodeGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NodeGroupVersionResource, NodeGroupVersionKind.Kind, false))

	return &nodeController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type nodeClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NodeController
}

func (s *nodeClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *nodeClient) Create(o *v1.Node) (*v1.Node, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Node), err
}

func (s *nodeClient) Get(name string, opts metav1.GetOptions) (*v1.Node, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Node), err
}

func (s *nodeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Node, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Node), err
}

func (s *nodeClient) Update(o *v1.Node) (*v1.Node, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Node), err
}

func (s *nodeClient) UpdateStatus(o *v1.Node) (*v1.Node, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Node), err
}

func (s *nodeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeClient) List(opts metav1.ListOptions) (*v1.NodeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.NodeList), err
}

func (s *nodeClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.NodeList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.NodeList), err
}

func (s *nodeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeClient) Patch(o *v1.Node, patchType types.PatchType, data []byte, subresources ...string) (*v1.Node, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Node), err
}

func (s *nodeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeClient) AddHandler(ctx context.Context, name string, sync NodeHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeClient) AddLifecycle(ctx context.Context, name string, lifecycle NodeLifecycle) {
	sync := NewNodeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeLifecycle) {
	sync := NewNodeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *nodeClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeLifecycle) {
	sync := NewNodeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeLifecycle) {
	sync := NewNodeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
