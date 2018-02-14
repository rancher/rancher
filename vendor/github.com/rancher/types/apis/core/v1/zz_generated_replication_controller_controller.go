package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
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
	ReplicationControllerGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ReplicationController",
	}
	ReplicationControllerResource = metav1.APIResource{
		Name:         "replicationcontrollers",
		SingularName: "replicationcontroller",
		Namespaced:   true,

		Kind: ReplicationControllerGroupVersionKind.Kind,
	}
)

type ReplicationControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ReplicationController
}

type ReplicationControllerHandlerFunc func(key string, obj *v1.ReplicationController) error

type ReplicationControllerLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ReplicationController, err error)
	Get(namespace, name string) (*v1.ReplicationController, error)
}

type ReplicationControllerController interface {
	Informer() cache.SharedIndexInformer
	Lister() ReplicationControllerLister
	AddHandler(name string, handler ReplicationControllerHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ReplicationControllerHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ReplicationControllerInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.ReplicationController) (*v1.ReplicationController, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicationController, error)
	Get(name string, opts metav1.GetOptions) (*v1.ReplicationController, error)
	Update(*v1.ReplicationController) (*v1.ReplicationController, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ReplicationControllerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ReplicationControllerController
	AddHandler(name string, sync ReplicationControllerHandlerFunc)
	AddLifecycle(name string, lifecycle ReplicationControllerLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ReplicationControllerHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ReplicationControllerLifecycle)
}

type replicationControllerLister struct {
	controller *replicationControllerController
}

func (l *replicationControllerLister) List(namespace string, selector labels.Selector) (ret []*v1.ReplicationController, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ReplicationController))
	})
	return
}

func (l *replicationControllerLister) Get(namespace, name string) (*v1.ReplicationController, error) {
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
			Group:    ReplicationControllerGroupVersionKind.Group,
			Resource: "replicationController",
		}, name)
	}
	return obj.(*v1.ReplicationController), nil
}

type replicationControllerController struct {
	controller.GenericController
}

func (c *replicationControllerController) Lister() ReplicationControllerLister {
	return &replicationControllerLister{
		controller: c,
	}
}

func (c *replicationControllerController) AddHandler(name string, handler ReplicationControllerHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.ReplicationController))
	})
}

func (c *replicationControllerController) AddClusterScopedHandler(name, cluster string, handler ReplicationControllerHandlerFunc) {
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

		return handler(key, obj.(*v1.ReplicationController))
	})
}

type replicationControllerFactory struct {
}

func (c replicationControllerFactory) Object() runtime.Object {
	return &v1.ReplicationController{}
}

func (c replicationControllerFactory) List() runtime.Object {
	return &ReplicationControllerList{}
}

func (s *replicationControllerClient) Controller() ReplicationControllerController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.replicationControllerControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ReplicationControllerGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &replicationControllerController{
		GenericController: genericController,
	}

	s.client.replicationControllerControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type replicationControllerClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ReplicationControllerController
}

func (s *replicationControllerClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *replicationControllerClient) Create(o *v1.ReplicationController) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) Get(name string, opts metav1.GetOptions) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) Update(o *v1.ReplicationController) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *replicationControllerClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *replicationControllerClient) List(opts metav1.ListOptions) (*ReplicationControllerList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ReplicationControllerList), err
}

func (s *replicationControllerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *replicationControllerClient) Patch(o *v1.ReplicationController, data []byte, subresources ...string) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *replicationControllerClient) AddHandler(name string, sync ReplicationControllerHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *replicationControllerClient) AddLifecycle(name string, lifecycle ReplicationControllerLifecycle) {
	sync := NewReplicationControllerLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *replicationControllerClient) AddClusterScopedHandler(name, clusterName string, sync ReplicationControllerHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *replicationControllerClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ReplicationControllerLifecycle) {
	sync := NewReplicationControllerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
