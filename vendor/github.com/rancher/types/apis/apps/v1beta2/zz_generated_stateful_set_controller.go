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
	StatefulSetGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "StatefulSet",
	}
	StatefulSetResource = metav1.APIResource{
		Name:         "statefulsets",
		SingularName: "statefulset",
		Namespaced:   true,

		Kind: StatefulSetGroupVersionKind.Kind,
	}
)

type StatefulSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta2.StatefulSet
}

type StatefulSetHandlerFunc func(key string, obj *v1beta2.StatefulSet) error

type StatefulSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta2.StatefulSet, err error)
	Get(namespace, name string) (*v1beta2.StatefulSet, error)
}

type StatefulSetController interface {
	Informer() cache.SharedIndexInformer
	Lister() StatefulSetLister
	AddHandler(name string, handler StatefulSetHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler StatefulSetHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type StatefulSetInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1beta2.StatefulSet) (*v1beta2.StatefulSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.StatefulSet, error)
	Get(name string, opts metav1.GetOptions) (*v1beta2.StatefulSet, error)
	Update(*v1beta2.StatefulSet) (*v1beta2.StatefulSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*StatefulSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() StatefulSetController
	AddHandler(name string, sync StatefulSetHandlerFunc)
	AddLifecycle(name string, lifecycle StatefulSetLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync StatefulSetHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle StatefulSetLifecycle)
}

type statefulSetLister struct {
	controller *statefulSetController
}

func (l *statefulSetLister) List(namespace string, selector labels.Selector) (ret []*v1beta2.StatefulSet, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta2.StatefulSet))
	})
	return
}

func (l *statefulSetLister) Get(namespace, name string) (*v1beta2.StatefulSet, error) {
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
			Group:    StatefulSetGroupVersionKind.Group,
			Resource: "statefulSet",
		}, name)
	}
	return obj.(*v1beta2.StatefulSet), nil
}

type statefulSetController struct {
	controller.GenericController
}

func (c *statefulSetController) Lister() StatefulSetLister {
	return &statefulSetLister{
		controller: c,
	}
}

func (c *statefulSetController) AddHandler(name string, handler StatefulSetHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1beta2.StatefulSet))
	})
}

func (c *statefulSetController) AddClusterScopedHandler(name, cluster string, handler StatefulSetHandlerFunc) {
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

		return handler(key, obj.(*v1beta2.StatefulSet))
	})
}

type statefulSetFactory struct {
}

func (c statefulSetFactory) Object() runtime.Object {
	return &v1beta2.StatefulSet{}
}

func (c statefulSetFactory) List() runtime.Object {
	return &StatefulSetList{}
}

func (s *statefulSetClient) Controller() StatefulSetController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.statefulSetControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(StatefulSetGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &statefulSetController{
		GenericController: genericController,
	}

	s.client.statefulSetControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type statefulSetClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   StatefulSetController
}

func (s *statefulSetClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *statefulSetClient) Create(o *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta2.StatefulSet), err
}

func (s *statefulSetClient) Get(name string, opts metav1.GetOptions) (*v1beta2.StatefulSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta2.StatefulSet), err
}

func (s *statefulSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.StatefulSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta2.StatefulSet), err
}

func (s *statefulSetClient) Update(o *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta2.StatefulSet), err
}

func (s *statefulSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *statefulSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *statefulSetClient) List(opts metav1.ListOptions) (*StatefulSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*StatefulSetList), err
}

func (s *statefulSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *statefulSetClient) Patch(o *v1beta2.StatefulSet, data []byte, subresources ...string) (*v1beta2.StatefulSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1beta2.StatefulSet), err
}

func (s *statefulSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *statefulSetClient) AddHandler(name string, sync StatefulSetHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *statefulSetClient) AddLifecycle(name string, lifecycle StatefulSetLifecycle) {
	sync := NewStatefulSetLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *statefulSetClient) AddClusterScopedHandler(name, clusterName string, sync StatefulSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *statefulSetClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle StatefulSetLifecycle) {
	sync := NewStatefulSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
