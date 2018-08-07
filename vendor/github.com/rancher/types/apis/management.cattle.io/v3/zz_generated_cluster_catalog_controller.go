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
	ClusterCatalogGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterCatalog",
	}
	ClusterCatalogResource = metav1.APIResource{
		Name:         "clustercatalogs",
		SingularName: "clustercatalog",
		Namespaced:   true,

		Kind: ClusterCatalogGroupVersionKind.Kind,
	}
)

type ClusterCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCatalog
}

type ClusterCatalogHandlerFunc func(key string, obj *ClusterCatalog) error

type ClusterCatalogLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterCatalog, err error)
	Get(namespace, name string) (*ClusterCatalog, error)
}

type ClusterCatalogController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterCatalogLister
	AddHandler(name string, handler ClusterCatalogHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterCatalogHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterCatalogInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterCatalog) (*ClusterCatalog, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterCatalog, error)
	Get(name string, opts metav1.GetOptions) (*ClusterCatalog, error)
	Update(*ClusterCatalog) (*ClusterCatalog, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterCatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterCatalogController
	AddHandler(name string, sync ClusterCatalogHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterCatalogLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterCatalogHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterCatalogLifecycle)
}

type clusterCatalogLister struct {
	controller *clusterCatalogController
}

func (l *clusterCatalogLister) List(namespace string, selector labels.Selector) (ret []*ClusterCatalog, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterCatalog))
	})
	return
}

func (l *clusterCatalogLister) Get(namespace, name string) (*ClusterCatalog, error) {
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
			Group:    ClusterCatalogGroupVersionKind.Group,
			Resource: "clusterCatalog",
		}, key)
	}
	return obj.(*ClusterCatalog), nil
}

type clusterCatalogController struct {
	controller.GenericController
}

func (c *clusterCatalogController) Lister() ClusterCatalogLister {
	return &clusterCatalogLister{
		controller: c,
	}
}

func (c *clusterCatalogController) AddHandler(name string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ClusterCatalog))
	})
}

func (c *clusterCatalogController) AddClusterScopedHandler(name, cluster string, handler ClusterCatalogHandlerFunc) {
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

		return handler(key, obj.(*ClusterCatalog))
	})
}

type clusterCatalogFactory struct {
}

func (c clusterCatalogFactory) Object() runtime.Object {
	return &ClusterCatalog{}
}

func (c clusterCatalogFactory) List() runtime.Object {
	return &ClusterCatalogList{}
}

func (s *clusterCatalogClient) Controller() ClusterCatalogController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterCatalogControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterCatalogGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterCatalogController{
		GenericController: genericController,
	}

	s.client.clusterCatalogControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterCatalogClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterCatalogController
}

func (s *clusterCatalogClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterCatalogClient) Create(o *ClusterCatalog) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) Get(name string, opts metav1.GetOptions) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterCatalog, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) Update(o *ClusterCatalog) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterCatalogClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterCatalogClient) List(opts metav1.ListOptions) (*ClusterCatalogList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterCatalogList), err
}

func (s *clusterCatalogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterCatalogClient) Patch(o *ClusterCatalog, data []byte, subresources ...string) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterCatalogClient) AddHandler(name string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterCatalogClient) AddLifecycle(name string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterCatalogClient) AddClusterScopedHandler(name, clusterName string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterCatalogClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
