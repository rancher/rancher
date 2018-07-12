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
	ClusterGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Cluster",
	}
	ClusterResource = metav1.APIResource{
		Name:         "clusters",
		SingularName: "cluster",
		Namespaced:   false,
		Kind:         ClusterGroupVersionKind.Kind,
	}
)

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster
}

type ClusterHandlerFunc func(key string, obj *Cluster) error

type ClusterLister interface {
	List(namespace string, selector labels.Selector) (ret []*Cluster, err error)
	Get(namespace, name string) (*Cluster, error)
}

type ClusterController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterLister
	AddHandler(name string, handler ClusterHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Cluster) (*Cluster, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Cluster, error)
	Get(name string, opts metav1.GetOptions) (*Cluster, error)
	Update(*Cluster) (*Cluster, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterController
	AddHandler(name string, sync ClusterHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterLifecycle)
}

type clusterLister struct {
	controller *clusterController
}

func (l *clusterLister) List(namespace string, selector labels.Selector) (ret []*Cluster, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Cluster))
	})
	return
}

func (l *clusterLister) Get(namespace, name string) (*Cluster, error) {
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
			Group:    ClusterGroupVersionKind.Group,
			Resource: "cluster",
		}, key)
	}
	return obj.(*Cluster), nil
}

type clusterController struct {
	controller.GenericController
}

func (c *clusterController) Lister() ClusterLister {
	return &clusterLister{
		controller: c,
	}
}

func (c *clusterController) AddHandler(name string, handler ClusterHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Cluster))
	})
}

func (c *clusterController) AddClusterScopedHandler(name, cluster string, handler ClusterHandlerFunc) {
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

		return handler(key, obj.(*Cluster))
	})
}

type clusterFactory struct {
}

func (c clusterFactory) Object() runtime.Object {
	return &Cluster{}
}

func (c clusterFactory) List() runtime.Object {
	return &ClusterList{}
}

func (s *clusterClient) Controller() ClusterController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterController{
		GenericController: genericController,
	}

	s.client.clusterControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterController
}

func (s *clusterClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterClient) Create(o *Cluster) (*Cluster, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Cluster), err
}

func (s *clusterClient) Get(name string, opts metav1.GetOptions) (*Cluster, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Cluster), err
}

func (s *clusterClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Cluster, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Cluster), err
}

func (s *clusterClient) Update(o *Cluster) (*Cluster, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Cluster), err
}

func (s *clusterClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterClient) List(opts metav1.ListOptions) (*ClusterList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterList), err
}

func (s *clusterClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterClient) Patch(o *Cluster, data []byte, subresources ...string) (*Cluster, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Cluster), err
}

func (s *clusterClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterClient) AddHandler(name string, sync ClusterHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterClient) AddLifecycle(name string, lifecycle ClusterLifecycle) {
	sync := NewClusterLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterClient) AddClusterScopedHandler(name, clusterName string, sync ClusterHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterLifecycle) {
	sync := NewClusterLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
