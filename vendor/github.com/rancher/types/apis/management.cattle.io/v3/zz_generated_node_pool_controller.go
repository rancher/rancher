package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePool
}

type NodePoolHandlerFunc func(key string, obj *NodePool) error

type NodePoolLister interface {
	List(namespace string, selector labels.Selector) (ret []*NodePool, err error)
	Get(namespace, name string) (*NodePool, error)
}

type NodePoolController interface {
	Informer() cache.SharedIndexInformer
	Lister() NodePoolLister
	AddHandler(name string, handler NodePoolHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NodePoolHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NodePoolInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NodePool) (*NodePool, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodePool, error)
	Get(name string, opts metav1.GetOptions) (*NodePool, error)
	Update(*NodePool) (*NodePool, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NodePoolList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodePoolController
	AddHandler(name string, sync NodePoolHandlerFunc)
	AddLifecycle(name string, lifecycle NodePoolLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NodePoolHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NodePoolLifecycle)
}

type nodePoolLister struct {
	controller *nodePoolController
}

func (l *nodePoolLister) List(namespace string, selector labels.Selector) (ret []*NodePool, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NodePool))
	})
	return
}

func (l *nodePoolLister) Get(namespace, name string) (*NodePool, error) {
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
			Resource: "nodePool",
		}, key)
	}
	return obj.(*NodePool), nil
}

type nodePoolController struct {
	controller.GenericController
}

func (c *nodePoolController) Lister() NodePoolLister {
	return &nodePoolLister{
		controller: c,
	}
}

func (c *nodePoolController) AddHandler(name string, handler NodePoolHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NodePool))
	})
}

func (c *nodePoolController) AddClusterScopedHandler(name, cluster string, handler NodePoolHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*NodePool))
	})
}

type nodePoolFactory struct {
}

func (c nodePoolFactory) Object() runtime.Object {
	return &NodePool{}
}

func (c nodePoolFactory) List() runtime.Object {
	return &NodePoolList{}
}

func (s *nodePoolClient) Controller() NodePoolController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.nodePoolControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NodePoolGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &nodePoolController{
		GenericController: genericController,
	}

	s.client.nodePoolControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *nodePoolClient) Create(o *NodePool) (*NodePool, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NodePool), err
}

func (s *nodePoolClient) Get(name string, opts metav1.GetOptions) (*NodePool, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NodePool), err
}

func (s *nodePoolClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodePool, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NodePool), err
}

func (s *nodePoolClient) Update(o *NodePool) (*NodePool, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NodePool), err
}

func (s *nodePoolClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodePoolClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodePoolClient) List(opts metav1.ListOptions) (*NodePoolList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NodePoolList), err
}

func (s *nodePoolClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodePoolClient) Patch(o *NodePool, data []byte, subresources ...string) (*NodePool, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NodePool), err
}

func (s *nodePoolClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodePoolClient) AddHandler(name string, sync NodePoolHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *nodePoolClient) AddLifecycle(name string, lifecycle NodePoolLifecycle) {
	sync := NewNodePoolLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *nodePoolClient) AddClusterScopedHandler(name, clusterName string, sync NodePoolHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *nodePoolClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NodePoolLifecycle) {
	sync := NewNodePoolLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
