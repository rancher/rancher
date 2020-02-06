package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

	WorkloadGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "workloads",
	}
)

func init() {
	resource.Put(WorkloadGroupVersionResource)
}

func NewWorkload(namespace, name string, obj Workload) *Workload {
	obj.APIVersion, obj.Kind = WorkloadGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload `json:"items"`
}

type WorkloadHandlerFunc func(key string, obj *Workload) (runtime.Object, error)

type WorkloadChangeHandlerFunc func(obj *Workload) (runtime.Object, error)

type WorkloadLister interface {
	List(namespace string, selector labels.Selector) (ret []*Workload, err error)
	Get(namespace, name string) (*Workload, error)
}

type WorkloadController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() WorkloadLister
	AddHandler(ctx context.Context, name string, handler WorkloadHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync WorkloadHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler WorkloadHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler WorkloadHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type WorkloadInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Workload) (*Workload, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Workload, error)
	Get(name string, opts metav1.GetOptions) (*Workload, error)
	Update(*Workload) (*Workload, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*WorkloadList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*WorkloadList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() WorkloadController
	AddHandler(ctx context.Context, name string, sync WorkloadHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync WorkloadHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle WorkloadLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle WorkloadLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync WorkloadHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync WorkloadHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle WorkloadLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle WorkloadLifecycle)
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
		}, key)
	}
	return obj.(*Workload), nil
}

type workloadController struct {
	controller.GenericController
}

func (c *workloadController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *workloadController) Lister() WorkloadLister {
	return &workloadLister{
		controller: c,
	}
}

func (c *workloadController) AddHandler(ctx context.Context, name string, handler WorkloadHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Workload); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *workloadController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler WorkloadHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Workload); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *workloadController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler WorkloadHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Workload); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *workloadController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler WorkloadHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Workload); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
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
	objectClient *objectclient.ObjectClient
	controller   WorkloadController
}

func (s *workloadClient) ObjectClient() *objectclient.ObjectClient {
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

func (s *workloadClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Workload, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Workload), err
}

func (s *workloadClient) Update(o *Workload) (*Workload, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Workload), err
}

func (s *workloadClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *workloadClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *workloadClient) List(opts metav1.ListOptions) (*WorkloadList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*WorkloadList), err
}

func (s *workloadClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*WorkloadList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*WorkloadList), err
}

func (s *workloadClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *workloadClient) Patch(o *Workload, patchType types.PatchType, data []byte, subresources ...string) (*Workload, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Workload), err
}

func (s *workloadClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *workloadClient) AddHandler(ctx context.Context, name string, sync WorkloadHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *workloadClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync WorkloadHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *workloadClient) AddLifecycle(ctx context.Context, name string, lifecycle WorkloadLifecycle) {
	sync := NewWorkloadLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *workloadClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle WorkloadLifecycle) {
	sync := NewWorkloadLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *workloadClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync WorkloadHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *workloadClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync WorkloadHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *workloadClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle WorkloadLifecycle) {
	sync := NewWorkloadLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *workloadClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle WorkloadLifecycle) {
	sync := NewWorkloadLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
