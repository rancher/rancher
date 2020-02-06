package v3

import (
	"context"
	"time"

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
	NodeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Node",
	}
	NodeResource = metav1.APIResource{
		Name:         "nodes",
		SingularName: "node",
		Namespaced:   true,

		Kind: NodeGroupVersionKind.Kind,
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

func NewNode(namespace, name string, obj Node) *Node {
	obj.APIVersion, obj.Kind = NodeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Node `json:"items"`
}

type NodeHandlerFunc func(key string, obj *Node) (runtime.Object, error)

type NodeChangeHandlerFunc func(obj *Node) (runtime.Object, error)

type NodeLister interface {
	List(namespace string, selector labels.Selector) (ret []*Node, err error)
	Get(namespace, name string) (*Node, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NodeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Node) (*Node, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Node, error)
	Get(name string, opts metav1.GetOptions) (*Node, error)
	Update(*Node) (*Node, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NodeList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*NodeList, error)
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
	controller *nodeController
}

func (l *nodeLister) List(namespace string, selector labels.Selector) (ret []*Node, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Node))
	})
	return
}

func (l *nodeLister) Get(namespace, name string) (*Node, error) {
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
			Resource: "node",
		}, key)
	}
	return obj.(*Node), nil
}

type nodeController struct {
	controller.GenericController
}

func (c *nodeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodeController) Lister() NodeLister {
	return &nodeLister{
		controller: c,
	}
}

func (c *nodeController) AddHandler(ctx context.Context, name string, handler NodeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Node); ok {
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
		} else if v, ok := obj.(*Node); ok {
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
		} else if v, ok := obj.(*Node); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*Node); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodeFactory struct {
}

func (c nodeFactory) Object() runtime.Object {
	return &Node{}
}

func (c nodeFactory) List() runtime.Object {
	return &NodeList{}
}

func (s *nodeClient) Controller() NodeController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.nodeControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NodeGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &nodeController{
		GenericController: genericController,
	}

	s.client.nodeControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *nodeClient) Create(o *Node) (*Node, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Node), err
}

func (s *nodeClient) Get(name string, opts metav1.GetOptions) (*Node, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Node), err
}

func (s *nodeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Node, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Node), err
}

func (s *nodeClient) Update(o *Node) (*Node, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Node), err
}

func (s *nodeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeClient) List(opts metav1.ListOptions) (*NodeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NodeList), err
}

func (s *nodeClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*NodeList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*NodeList), err
}

func (s *nodeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeClient) Patch(o *Node, patchType types.PatchType, data []byte, subresources ...string) (*Node, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Node), err
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
