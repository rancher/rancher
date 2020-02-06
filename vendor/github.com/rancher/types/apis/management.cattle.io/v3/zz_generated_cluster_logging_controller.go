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
	ClusterLoggingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterLogging",
	}
	ClusterLoggingResource = metav1.APIResource{
		Name:         "clusterloggings",
		SingularName: "clusterlogging",
		Namespaced:   true,

		Kind: ClusterLoggingGroupVersionKind.Kind,
	}

	ClusterLoggingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterloggings",
	}
)

func init() {
	resource.Put(ClusterLoggingGroupVersionResource)
}

func NewClusterLogging(namespace, name string, obj ClusterLogging) *ClusterLogging {
	obj.APIVersion, obj.Kind = ClusterLoggingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterLoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterLogging `json:"items"`
}

type ClusterLoggingHandlerFunc func(key string, obj *ClusterLogging) (runtime.Object, error)

type ClusterLoggingChangeHandlerFunc func(obj *ClusterLogging) (runtime.Object, error)

type ClusterLoggingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterLogging, err error)
	Get(namespace, name string) (*ClusterLogging, error)
}

type ClusterLoggingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterLoggingLister
	AddHandler(ctx context.Context, name string, handler ClusterLoggingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterLoggingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterLoggingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterLoggingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterLoggingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterLogging) (*ClusterLogging, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterLogging, error)
	Get(name string, opts metav1.GetOptions) (*ClusterLogging, error)
	Update(*ClusterLogging) (*ClusterLogging, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterLoggingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterLoggingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterLoggingController
	AddHandler(ctx context.Context, name string, sync ClusterLoggingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterLoggingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterLoggingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterLoggingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterLoggingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterLoggingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterLoggingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterLoggingLifecycle)
}

type clusterLoggingLister struct {
	controller *clusterLoggingController
}

func (l *clusterLoggingLister) List(namespace string, selector labels.Selector) (ret []*ClusterLogging, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterLogging))
	})
	return
}

func (l *clusterLoggingLister) Get(namespace, name string) (*ClusterLogging, error) {
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
			Group:    ClusterLoggingGroupVersionKind.Group,
			Resource: "clusterLogging",
		}, key)
	}
	return obj.(*ClusterLogging), nil
}

type clusterLoggingController struct {
	controller.GenericController
}

func (c *clusterLoggingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterLoggingController) Lister() ClusterLoggingLister {
	return &clusterLoggingLister{
		controller: c,
	}
}

func (c *clusterLoggingController) AddHandler(ctx context.Context, name string, handler ClusterLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterLogging); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterLoggingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterLogging); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterLoggingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterLogging); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterLoggingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterLogging); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterLoggingFactory struct {
}

func (c clusterLoggingFactory) Object() runtime.Object {
	return &ClusterLogging{}
}

func (c clusterLoggingFactory) List() runtime.Object {
	return &ClusterLoggingList{}
}

func (s *clusterLoggingClient) Controller() ClusterLoggingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterLoggingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterLoggingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterLoggingController{
		GenericController: genericController,
	}

	s.client.clusterLoggingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterLoggingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterLoggingController
}

func (s *clusterLoggingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterLoggingClient) Create(o *ClusterLogging) (*ClusterLogging, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) Get(name string, opts metav1.GetOptions) (*ClusterLogging, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterLogging, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) Update(o *ClusterLogging) (*ClusterLogging, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterLoggingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterLoggingClient) List(opts metav1.ListOptions) (*ClusterLoggingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterLoggingList), err
}

func (s *clusterLoggingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterLoggingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterLoggingList), err
}

func (s *clusterLoggingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterLoggingClient) Patch(o *ClusterLogging, patchType types.PatchType, data []byte, subresources ...string) (*ClusterLogging, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterLogging), err
}

func (s *clusterLoggingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterLoggingClient) AddHandler(ctx context.Context, name string, sync ClusterLoggingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterLoggingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterLoggingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterLoggingClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterLoggingLifecycle) {
	sync := NewClusterLoggingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterLoggingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterLoggingLifecycle) {
	sync := NewClusterLoggingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterLoggingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterLoggingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterLoggingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterLoggingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterLoggingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterLoggingLifecycle) {
	sync := NewClusterLoggingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterLoggingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterLoggingLifecycle) {
	sync := NewClusterLoggingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
