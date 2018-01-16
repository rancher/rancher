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
	WorkloadGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Workload",
	}
	WorkloadResource = metav1.APIResource{
		Name:         "workloads",
		SingularName: "workload",
		Namespaced:   true,

		Kind: WorkloadGroupVersionKind.Kind,
	}
)

type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload
}

type WorkloadHandlerFunc func(key string, obj *Workload) error

type WorkloadLister interface {
	List(namespace string, selector labels.Selector) (ret []*Workload, err error)
	Get(namespace, name string) (*Workload, error)
}

type WorkloadController interface {
	Informer() cache.SharedIndexInformer
	Lister() WorkloadLister
	AddHandler(name string, handler WorkloadHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler WorkloadHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type WorkloadInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Workload) (*Workload, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*Workload, error)
	Get(name string, opts metav1.GetOptions) (*Workload, error)
	Update(*Workload) (*Workload, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*WorkloadList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() WorkloadController
	AddHandler(name string, sync WorkloadHandlerFunc)
	AddLifecycle(name string, lifecycle WorkloadLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync WorkloadHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle WorkloadLifecycle)
}

type workloadLister struct {
	controller *workloadController
}

func (l *workloadLister) List(namespace string, selector labels.Selector) (ret []*Workload, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Workload))
	})
	return
}

func (l *workloadLister) Get(namespace, name string) (*Workload, error) {
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
			Group:    WorkloadGroupVersionKind.Group,
			Resource: "workload",
		}, name)
	}
	return obj.(*Workload), nil
}

type workloadController struct {
	controller.GenericController
}

func (c *workloadController) Lister() WorkloadLister {
	return &workloadLister{
		controller: c,
	}
}

func (c *workloadController) AddHandler(name string, handler WorkloadHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Workload))
	})
}

func (c *workloadController) AddClusterScopedHandler(name, cluster string, handler WorkloadHandlerFunc) {
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

		return handler(key, obj.(*Workload))
	})
}

type workloadFactory struct {
}

func (c workloadFactory) Object() runtime.Object {
	return &Workload{}
}

func (c workloadFactory) List() runtime.Object {
	return &WorkloadList{}
}

func (s *workloadClient) Controller() WorkloadController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.workloadControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(WorkloadGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &workloadController{
		GenericController: genericController,
	}

	s.client.workloadControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type workloadClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   WorkloadController
}

func (s *workloadClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *workloadClient) Create(o *Workload) (*Workload, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Workload), err
}

func (s *workloadClient) Get(name string, opts metav1.GetOptions) (*Workload, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Workload), err
}

func (s *workloadClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*Workload, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*Workload), err
}

func (s *workloadClient) Update(o *Workload) (*Workload, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Workload), err
}

func (s *workloadClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *workloadClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *workloadClient) List(opts metav1.ListOptions) (*WorkloadList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*WorkloadList), err
}

func (s *workloadClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *workloadClient) Patch(o *Workload, data []byte, subresources ...string) (*Workload, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Workload), err
}

func (s *workloadClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *workloadClient) AddHandler(name string, sync WorkloadHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *workloadClient) AddLifecycle(name string, lifecycle WorkloadLifecycle) {
	sync := NewWorkloadLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *workloadClient) AddClusterScopedHandler(name, clusterName string, sync WorkloadHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *workloadClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle WorkloadLifecycle) {
	sync := NewWorkloadLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
