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

	ClusterAlertGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusteralerts",
	}
)

func init() {
	resource.Put(ClusterAlertGroupVersionResource)
}

func NewClusterAlert(namespace, name string, obj ClusterAlert) *ClusterAlert {
	obj.APIVersion, obj.Kind = ClusterAlertGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAlert `json:"items"`
}

type ClusterAlertHandlerFunc func(key string, obj *ClusterAlert) (runtime.Object, error)

type ClusterAlertChangeHandlerFunc func(obj *ClusterAlert) (runtime.Object, error)

type ClusterAlertLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterAlert, err error)
	Get(namespace, name string) (*ClusterAlert, error)
}

type ClusterAlertController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterAlertLister
	AddHandler(ctx context.Context, name string, handler ClusterAlertHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterAlertHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterAlertHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterAlertInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterAlert) (*ClusterAlert, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAlert, error)
	Get(name string, opts metav1.GetOptions) (*ClusterAlert, error)
	Update(*ClusterAlert) (*ClusterAlert, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterAlertList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterAlertList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterAlertController
	AddHandler(ctx context.Context, name string, sync ClusterAlertHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterAlertLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAlertLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAlertHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAlertHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAlertLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAlertLifecycle)
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
		}, key)
	}
	return obj.(*ClusterAlert), nil
}

type clusterAlertController struct {
	controller.GenericController
}

func (c *clusterAlertController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterAlertController) Lister() ClusterAlertLister {
	return &clusterAlertLister{
		controller: c,
	}
}

func (c *clusterAlertController) AddHandler(ctx context.Context, name string, handler ClusterAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAlert); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAlert); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAlert); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAlert); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
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
	objectClient *objectclient.ObjectClient
	controller   ClusterAlertController
}

func (s *clusterAlertClient) ObjectClient() *objectclient.ObjectClient {
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

func (s *clusterAlertClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterAlertList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterAlertList), err
}

func (s *clusterAlertClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAlertClient) Patch(o *ClusterAlert, patchType types.PatchType, data []byte, subresources ...string) (*ClusterAlert, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterAlert), err
}

func (s *clusterAlertClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterAlertClient) AddHandler(ctx context.Context, name string, sync ClusterAlertHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAlertClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAlertClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterAlertLifecycle) {
	sync := NewClusterAlertLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAlertClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAlertLifecycle) {
	sync := NewClusterAlertLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAlertClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAlertHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAlertClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAlertHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterAlertClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAlertLifecycle) {
	sync := NewClusterAlertLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAlertClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAlertLifecycle) {
	sync := NewClusterAlertLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
