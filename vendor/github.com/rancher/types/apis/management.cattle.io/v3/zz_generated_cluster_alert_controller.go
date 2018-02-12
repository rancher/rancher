package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ClusterAlertGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterAlert",
	}
	ClusterAlertResource = metav1.APIResource{
		Name:         "clusteralerts",
		SingularName: "clusteralert",
		Namespaced:   true,

		Kind: ClusterAlertGroupVersionKind.Kind,
	}
)

type ClusterAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAlert
}

type ClusterAlertHandlerFunc func(key string, obj *ClusterAlert) error

type ClusterAlertLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterAlert, err error)
	Get(namespace, name string) (*ClusterAlert, error)
}

type ClusterAlertController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterAlertLister
	AddHandler(name string, handler ClusterAlertHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterAlertHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterAlertInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*ClusterAlert) (*ClusterAlert, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAlert, error)
	Get(name string, opts metav1.GetOptions) (*ClusterAlert, error)
	Update(*ClusterAlert) (*ClusterAlert, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterAlertList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterAlertController
	AddHandler(name string, sync ClusterAlertHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterAlertLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterAlertHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterAlertLifecycle)
}

type clusterAlertLister struct {
	controller *clusterAlertController
}

func (l *clusterAlertLister) List(namespace string, selector labels.Selector) (ret []*ClusterAlert, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterAlert))
	})
	return
}

func (l *clusterAlertLister) Get(namespace, name string) (*ClusterAlert, error) {
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
			Group:    ClusterAlertGroupVersionKind.Group,
			Resource: "clusterAlert",
		}, name)
	}
	return obj.(*ClusterAlert), nil
}

type clusterAlertController struct {
	controller.GenericController
}

func (c *clusterAlertController) Lister() ClusterAlertLister {
	return &clusterAlertLister{
		controller: c,
	}
}

func (c *clusterAlertController) AddHandler(name string, handler ClusterAlertHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ClusterAlert))
	})
}

func (c *clusterAlertController) AddClusterScopedHandler(name, cluster string, handler ClusterAlertHandlerFunc) {
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

		return handler(key, obj.(*ClusterAlert))
	})
}

type clusterAlertFactory struct {
}

func (c clusterAlertFactory) Object() runtime.Object {
	return &ClusterAlert{}
}

func (c clusterAlertFactory) List() runtime.Object {
	return &ClusterAlertList{}
}

func (s *clusterAlertClient) Controller() ClusterAlertController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterAlertControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterAlertGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterAlertController{
		GenericController: genericController,
	}

	s.client.clusterAlertControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterAlertClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ClusterAlertController
}

func (s *clusterAlertClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *clusterAlertClient) Create(o *ClusterAlert) (*ClusterAlert, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterAlert), err
}

func (s *clusterAlertClient) Get(name string, opts metav1.GetOptions) (*ClusterAlert, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterAlert), err
}

func (s *clusterAlertClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAlert, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterAlert), err
}

func (s *clusterAlertClient) Update(o *ClusterAlert) (*ClusterAlert, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterAlert), err
}

func (s *clusterAlertClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterAlertClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterAlertClient) List(opts metav1.ListOptions) (*ClusterAlertList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterAlertList), err
}

func (s *clusterAlertClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAlertClient) Patch(o *ClusterAlert, data []byte, subresources ...string) (*ClusterAlert, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ClusterAlert), err
}

func (s *clusterAlertClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterAlertClient) AddHandler(name string, sync ClusterAlertHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterAlertClient) AddLifecycle(name string, lifecycle ClusterAlertLifecycle) {
	sync := NewClusterAlertLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterAlertClient) AddClusterScopedHandler(name, clusterName string, sync ClusterAlertHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterAlertClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterAlertLifecycle) {
	sync := NewClusterAlertLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
