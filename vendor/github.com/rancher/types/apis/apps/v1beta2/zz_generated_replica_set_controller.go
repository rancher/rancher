package v1beta2

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
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
	ReplicaSetGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ReplicaSet",
	}
	ReplicaSetResource = metav1.APIResource{
		Name:         "replicasets",
		SingularName: "replicaset",
		Namespaced:   true,

		Kind: ReplicaSetGroupVersionKind.Kind,
	}
)

type ReplicaSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta2.ReplicaSet
}

type ReplicaSetHandlerFunc func(key string, obj *v1beta2.ReplicaSet) error

type ReplicaSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta2.ReplicaSet, err error)
	Get(namespace, name string) (*v1beta2.ReplicaSet, error)
}

type ReplicaSetController interface {
	Informer() cache.SharedIndexInformer
	Lister() ReplicaSetLister
	AddHandler(name string, handler ReplicaSetHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ReplicaSetHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ReplicaSetInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.ReplicaSet, error)
	Get(name string, opts metav1.GetOptions) (*v1beta2.ReplicaSet, error)
	Update(*v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ReplicaSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ReplicaSetController
	AddHandler(name string, sync ReplicaSetHandlerFunc)
	AddLifecycle(name string, lifecycle ReplicaSetLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ReplicaSetHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ReplicaSetLifecycle)
}

type replicaSetLister struct {
	controller *replicaSetController
}

func (l *replicaSetLister) List(namespace string, selector labels.Selector) (ret []*v1beta2.ReplicaSet, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta2.ReplicaSet))
	})
	return
}

func (l *replicaSetLister) Get(namespace, name string) (*v1beta2.ReplicaSet, error) {
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
			Group:    ReplicaSetGroupVersionKind.Group,
			Resource: "replicaSet",
		}, name)
	}
	return obj.(*v1beta2.ReplicaSet), nil
}

type replicaSetController struct {
	controller.GenericController
}

func (c *replicaSetController) Lister() ReplicaSetLister {
	return &replicaSetLister{
		controller: c,
	}
}

func (c *replicaSetController) AddHandler(name string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1beta2.ReplicaSet))
	})
}

func (c *replicaSetController) AddClusterScopedHandler(name, cluster string, handler ReplicaSetHandlerFunc) {
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

		return handler(key, obj.(*v1beta2.ReplicaSet))
	})
}

type replicaSetFactory struct {
}

func (c replicaSetFactory) Object() runtime.Object {
	return &v1beta2.ReplicaSet{}
}

func (c replicaSetFactory) List() runtime.Object {
	return &ReplicaSetList{}
}

func (s *replicaSetClient) Controller() ReplicaSetController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.replicaSetControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ReplicaSetGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &replicaSetController{
		GenericController: genericController,
	}

	s.client.replicaSetControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type replicaSetClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ReplicaSetController
}

func (s *replicaSetClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *replicaSetClient) Create(o *v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta2.ReplicaSet), err
}

func (s *replicaSetClient) Get(name string, opts metav1.GetOptions) (*v1beta2.ReplicaSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta2.ReplicaSet), err
}

func (s *replicaSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.ReplicaSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta2.ReplicaSet), err
}

func (s *replicaSetClient) Update(o *v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta2.ReplicaSet), err
}

func (s *replicaSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *replicaSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *replicaSetClient) List(opts metav1.ListOptions) (*ReplicaSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ReplicaSetList), err
}

func (s *replicaSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *replicaSetClient) Patch(o *v1beta2.ReplicaSet, data []byte, subresources ...string) (*v1beta2.ReplicaSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1beta2.ReplicaSet), err
}

func (s *replicaSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *replicaSetClient) AddHandler(name string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *replicaSetClient) AddLifecycle(name string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *replicaSetClient) AddClusterScopedHandler(name, clusterName string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
