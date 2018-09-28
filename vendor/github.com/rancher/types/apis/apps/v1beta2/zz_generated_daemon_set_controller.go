package v1beta2

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	DaemonSetGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DaemonSet",
	}
	DaemonSetResource = metav1.APIResource{
		Name:         "daemonsets",
		SingularName: "daemonset",
		Namespaced:   true,

		Kind: DaemonSetGroupVersionKind.Kind,
	}
)

type DaemonSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta2.DaemonSet
}

type DaemonSetHandlerFunc func(key string, obj *v1beta2.DaemonSet) error

type DaemonSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta2.DaemonSet, err error)
	Get(namespace, name string) (*v1beta2.DaemonSet, error)
}

type DaemonSetController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DaemonSetLister
	AddHandler(name string, handler DaemonSetHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler DaemonSetHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DaemonSetInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1beta2.DaemonSet) (*v1beta2.DaemonSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.DaemonSet, error)
	Get(name string, opts metav1.GetOptions) (*v1beta2.DaemonSet, error)
	Update(*v1beta2.DaemonSet) (*v1beta2.DaemonSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DaemonSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DaemonSetController
	AddHandler(name string, sync DaemonSetHandlerFunc)
	AddLifecycle(name string, lifecycle DaemonSetLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync DaemonSetHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle DaemonSetLifecycle)
}

type daemonSetLister struct {
	controller *daemonSetController
}

func (l *daemonSetLister) List(namespace string, selector labels.Selector) (ret []*v1beta2.DaemonSet, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta2.DaemonSet))
	})
	return
}

func (l *daemonSetLister) Get(namespace, name string) (*v1beta2.DaemonSet, error) {
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
			Group:    DaemonSetGroupVersionKind.Group,
			Resource: "daemonSet",
		}, key)
	}
	return obj.(*v1beta2.DaemonSet), nil
}

type daemonSetController struct {
	controller.GenericController
}

func (c *daemonSetController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *daemonSetController) Lister() DaemonSetLister {
	return &daemonSetLister{
		controller: c,
	}
}

func (c *daemonSetController) AddHandler(name string, handler DaemonSetHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1beta2.DaemonSet))
	})
}

func (c *daemonSetController) AddClusterScopedHandler(name, cluster string, handler DaemonSetHandlerFunc) {
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

		return handler(key, obj.(*v1beta2.DaemonSet))
	})
}

type daemonSetFactory struct {
}

func (c daemonSetFactory) Object() runtime.Object {
	return &v1beta2.DaemonSet{}
}

func (c daemonSetFactory) List() runtime.Object {
	return &DaemonSetList{}
}

func (s *daemonSetClient) Controller() DaemonSetController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.daemonSetControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DaemonSetGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &daemonSetController{
		GenericController: genericController,
	}

	s.client.daemonSetControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type daemonSetClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DaemonSetController
}

func (s *daemonSetClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *daemonSetClient) Create(o *v1beta2.DaemonSet) (*v1beta2.DaemonSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta2.DaemonSet), err
}

func (s *daemonSetClient) Get(name string, opts metav1.GetOptions) (*v1beta2.DaemonSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta2.DaemonSet), err
}

func (s *daemonSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.DaemonSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta2.DaemonSet), err
}

func (s *daemonSetClient) Update(o *v1beta2.DaemonSet) (*v1beta2.DaemonSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta2.DaemonSet), err
}

func (s *daemonSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *daemonSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *daemonSetClient) List(opts metav1.ListOptions) (*DaemonSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DaemonSetList), err
}

func (s *daemonSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *daemonSetClient) Patch(o *v1beta2.DaemonSet, data []byte, subresources ...string) (*v1beta2.DaemonSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1beta2.DaemonSet), err
}

func (s *daemonSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *daemonSetClient) AddHandler(name string, sync DaemonSetHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *daemonSetClient) AddLifecycle(name string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *daemonSetClient) AddClusterScopedHandler(name, clusterName string, sync DaemonSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *daemonSetClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle DaemonSetLifecycle) {
	sync := NewDaemonSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
