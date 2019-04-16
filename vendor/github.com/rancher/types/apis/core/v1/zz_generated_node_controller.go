package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Node
}

type NodeHandlerFunc func(key string, obj *v1.Node) error

type NodeLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Node, err error)
	Get(namespace, name string) (*v1.Node, error)
}

type NodeController interface {
	Informer() cache.SharedIndexInformer
	Lister() NodeLister
	AddHandler(name string, handler NodeHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NodeHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NodeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Node) (*v1.Node, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Node, error)
	Get(name string, opts metav1.GetOptions) (*v1.Node, error)
	Update(*v1.Node) (*v1.Node, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NodeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeController
	AddHandler(name string, sync NodeHandlerFunc)
	AddLifecycle(name string, lifecycle NodeLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NodeHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NodeLifecycle)
}

type nodeLister struct {
	controller *nodeController
}

func (l *nodeLister) List(namespace string, selector labels.Selector) (ret []*v1.Node, err error) {
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
			Resource: "node",
		}, key)
	}
	return obj.(*v1.Node), nil
}

type nodeController struct {
	controller.GenericController
}

func (c *nodeController) Lister() NodeLister {
	return &nodeLister{
		controller: c,
	}
}

func (c *nodeController) AddHandler(name string, handler NodeHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Node))
	})
}

func (c *nodeController) AddClusterScopedHandler(name, cluster string, handler NodeHandlerFunc) {
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

		return handler(key, obj.(*v1.Node))
	})
}

type nodeFactory struct {
}

func (c nodeFactory) Object() runtime.Object {
	return &v1.Node{}
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

func (s *nodeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeClient) Patch(o *v1.Node, data []byte, subresources ...string) (*v1.Node, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.Node), err
}

func (s *nodeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeClient) AddHandler(name string, sync NodeHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *nodeClient) AddLifecycle(name string, lifecycle NodeLifecycle) {
	sync := NewNodeLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *nodeClient) AddClusterScopedHandler(name, clusterName string, sync NodeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *nodeClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NodeLifecycle) {
	sync := NewNodeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
