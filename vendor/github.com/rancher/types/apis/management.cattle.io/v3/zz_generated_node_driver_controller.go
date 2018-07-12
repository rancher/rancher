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
)

type NodeDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeDriver
}

type NodeDriverHandlerFunc func(key string, obj *NodeDriver) error

type NodeDriverLister interface {
	List(namespace string, selector labels.Selector) (ret []*NodeDriver, err error)
	Get(namespace, name string) (*NodeDriver, error)
}

type NodeDriverController interface {
	Informer() cache.SharedIndexInformer
	Lister() NodeDriverLister
	AddHandler(name string, handler NodeDriverHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NodeDriverHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NodeDriverInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NodeDriver) (*NodeDriver, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodeDriver, error)
	Get(name string, opts metav1.GetOptions) (*NodeDriver, error)
	Update(*NodeDriver) (*NodeDriver, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NodeDriverList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeDriverController
	AddHandler(name string, sync NodeDriverHandlerFunc)
	AddLifecycle(name string, lifecycle NodeDriverLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NodeDriverHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NodeDriverLifecycle)
}

type nodeDriverLister struct {
	controller *nodeDriverController
}

func (l *nodeDriverLister) List(namespace string, selector labels.Selector) (ret []*NodeDriver, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NodeDriver))
	})
	return
}

func (l *nodeDriverLister) Get(namespace, name string) (*NodeDriver, error) {
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
			Resource: "nodeDriver",
		}, key)
	}
	return obj.(*NodeDriver), nil
}

type nodeDriverController struct {
	controller.GenericController
}

func (c *nodeDriverController) Lister() NodeDriverLister {
	return &nodeDriverLister{
		controller: c,
	}
}

func (c *nodeDriverController) AddHandler(name string, handler NodeDriverHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NodeDriver))
	})
}

func (c *nodeDriverController) AddClusterScopedHandler(name, cluster string, handler NodeDriverHandlerFunc) {
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

		return handler(key, obj.(*NodeDriver))
	})
}

type nodeDriverFactory struct {
}

func (c nodeDriverFactory) Object() runtime.Object {
	return &NodeDriver{}
}

func (c nodeDriverFactory) List() runtime.Object {
	return &NodeDriverList{}
}

func (s *nodeDriverClient) Controller() NodeDriverController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.nodeDriverControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NodeDriverGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &nodeDriverController{
		GenericController: genericController,
	}

	s.client.nodeDriverControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *nodeDriverClient) Create(o *NodeDriver) (*NodeDriver, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) Get(name string, opts metav1.GetOptions) (*NodeDriver, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodeDriver, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) Update(o *NodeDriver) (*NodeDriver, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeDriverClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeDriverClient) List(opts metav1.ListOptions) (*NodeDriverList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NodeDriverList), err
}

func (s *nodeDriverClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeDriverClient) Patch(o *NodeDriver, data []byte, subresources ...string) (*NodeDriver, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NodeDriver), err
}

func (s *nodeDriverClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeDriverClient) AddHandler(name string, sync NodeDriverHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *nodeDriverClient) AddLifecycle(name string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *nodeDriverClient) AddClusterScopedHandler(name, clusterName string, sync NodeDriverHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *nodeDriverClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NodeDriverLifecycle) {
	sync := NewNodeDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
