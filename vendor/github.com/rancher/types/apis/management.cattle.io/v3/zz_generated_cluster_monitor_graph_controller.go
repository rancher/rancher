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
	ClusterMonitorGraphGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterMonitorGraph",
	}
	ClusterMonitorGraphResource = metav1.APIResource{
		Name:         "clustermonitorgraphs",
		SingularName: "clustermonitorgraph",
		Namespaced:   true,

		Kind: ClusterMonitorGraphGroupVersionKind.Kind,
	}

	ClusterMonitorGraphGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clustermonitorgraphs",
	}
)

func init() {
	resource.Put(ClusterMonitorGraphGroupVersionResource)
}

func NewClusterMonitorGraph(namespace, name string, obj ClusterMonitorGraph) *ClusterMonitorGraph {
	obj.APIVersion, obj.Kind = ClusterMonitorGraphGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterMonitorGraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterMonitorGraph `json:"items"`
}

type ClusterMonitorGraphHandlerFunc func(key string, obj *ClusterMonitorGraph) (runtime.Object, error)

type ClusterMonitorGraphChangeHandlerFunc func(obj *ClusterMonitorGraph) (runtime.Object, error)

type ClusterMonitorGraphLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterMonitorGraph, err error)
	Get(namespace, name string) (*ClusterMonitorGraph, error)
}

type ClusterMonitorGraphController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterMonitorGraphLister
	AddHandler(ctx context.Context, name string, handler ClusterMonitorGraphHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterMonitorGraphHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterMonitorGraphHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterMonitorGraphHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterMonitorGraphInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterMonitorGraph) (*ClusterMonitorGraph, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterMonitorGraph, error)
	Get(name string, opts metav1.GetOptions) (*ClusterMonitorGraph, error)
	Update(*ClusterMonitorGraph) (*ClusterMonitorGraph, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterMonitorGraphList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterMonitorGraphList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterMonitorGraphController
	AddHandler(ctx context.Context, name string, sync ClusterMonitorGraphHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterMonitorGraphHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterMonitorGraphLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterMonitorGraphLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterMonitorGraphHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterMonitorGraphHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterMonitorGraphLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterMonitorGraphLifecycle)
}

type clusterMonitorGraphLister struct {
	controller *clusterMonitorGraphController
}

func (l *clusterMonitorGraphLister) List(namespace string, selector labels.Selector) (ret []*ClusterMonitorGraph, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterMonitorGraph))
	})
	return
}

func (l *clusterMonitorGraphLister) Get(namespace, name string) (*ClusterMonitorGraph, error) {
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
			Group:    ClusterMonitorGraphGroupVersionKind.Group,
			Resource: "clusterMonitorGraph",
		}, key)
	}
	return obj.(*ClusterMonitorGraph), nil
}

type clusterMonitorGraphController struct {
	controller.GenericController
}

func (c *clusterMonitorGraphController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterMonitorGraphController) Lister() ClusterMonitorGraphLister {
	return &clusterMonitorGraphLister{
		controller: c,
	}
}

func (c *clusterMonitorGraphController) AddHandler(ctx context.Context, name string, handler ClusterMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterMonitorGraph); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterMonitorGraphController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterMonitorGraph); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterMonitorGraphController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterMonitorGraph); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterMonitorGraphController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterMonitorGraph); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterMonitorGraphFactory struct {
}

func (c clusterMonitorGraphFactory) Object() runtime.Object {
	return &ClusterMonitorGraph{}
}

func (c clusterMonitorGraphFactory) List() runtime.Object {
	return &ClusterMonitorGraphList{}
}

func (s *clusterMonitorGraphClient) Controller() ClusterMonitorGraphController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterMonitorGraphControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterMonitorGraphGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterMonitorGraphController{
		GenericController: genericController,
	}

	s.client.clusterMonitorGraphControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterMonitorGraphClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterMonitorGraphController
}

func (s *clusterMonitorGraphClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterMonitorGraphClient) Create(o *ClusterMonitorGraph) (*ClusterMonitorGraph, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterMonitorGraph), err
}

func (s *clusterMonitorGraphClient) Get(name string, opts metav1.GetOptions) (*ClusterMonitorGraph, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterMonitorGraph), err
}

func (s *clusterMonitorGraphClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterMonitorGraph, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterMonitorGraph), err
}

func (s *clusterMonitorGraphClient) Update(o *ClusterMonitorGraph) (*ClusterMonitorGraph, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterMonitorGraph), err
}

func (s *clusterMonitorGraphClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterMonitorGraphClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterMonitorGraphClient) List(opts metav1.ListOptions) (*ClusterMonitorGraphList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterMonitorGraphList), err
}

func (s *clusterMonitorGraphClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterMonitorGraphList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterMonitorGraphList), err
}

func (s *clusterMonitorGraphClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterMonitorGraphClient) Patch(o *ClusterMonitorGraph, patchType types.PatchType, data []byte, subresources ...string) (*ClusterMonitorGraph, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterMonitorGraph), err
}

func (s *clusterMonitorGraphClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterMonitorGraphClient) AddHandler(ctx context.Context, name string, sync ClusterMonitorGraphHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterMonitorGraphClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterMonitorGraphHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterMonitorGraphClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterMonitorGraphLifecycle) {
	sync := NewClusterMonitorGraphLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterMonitorGraphClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterMonitorGraphLifecycle) {
	sync := NewClusterMonitorGraphLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterMonitorGraphClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterMonitorGraphHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterMonitorGraphClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterMonitorGraphHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterMonitorGraphClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterMonitorGraphLifecycle) {
	sync := NewClusterMonitorGraphLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterMonitorGraphClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterMonitorGraphLifecycle) {
	sync := NewClusterMonitorGraphLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
