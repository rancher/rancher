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

	ClusterCatalogGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clustercatalogs",
	}
)

func init() {
	resource.Put(ClusterCatalogGroupVersionResource)
}

func NewClusterCatalog(namespace, name string, obj ClusterCatalog) *ClusterCatalog {
	obj.APIVersion, obj.Kind = ClusterCatalogGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCatalog `json:"items"`
}

type ClusterCatalogHandlerFunc func(key string, obj *ClusterCatalog) (runtime.Object, error)

type ClusterCatalogChangeHandlerFunc func(obj *ClusterCatalog) (runtime.Object, error)

type ClusterCatalogLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterCatalog, err error)
	Get(namespace, name string) (*ClusterCatalog, error)
}

type ClusterCatalogController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterCatalogLister
	AddHandler(ctx context.Context, name string, handler ClusterCatalogHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterCatalogHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterCatalogHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterCatalogHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
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
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterCatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterCatalogController
	AddHandler(ctx context.Context, name string, sync ClusterCatalogHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterCatalogHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterCatalogLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterCatalogLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterCatalogHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterCatalogHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterCatalogLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterCatalogLifecycle)
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

func (c *clusterCatalogController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterCatalogController) Lister() ClusterCatalogLister {
	return &clusterCatalogLister{
		controller: c,
	}
}

func (c *clusterCatalogController) AddHandler(ctx context.Context, name string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterCatalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterCatalogController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterCatalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterCatalogController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterCatalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterCatalogController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterCatalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
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

func (s *clusterCatalogClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterCatalogList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterCatalogList), err
}

func (s *clusterCatalogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterCatalogClient) Patch(o *ClusterCatalog, patchType types.PatchType, data []byte, subresources ...string) (*ClusterCatalog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterCatalog), err
}

func (s *clusterCatalogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterCatalogClient) AddHandler(ctx context.Context, name string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterCatalogClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterCatalogClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterCatalogClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterCatalogClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterCatalogClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterCatalogHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterCatalogClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterCatalogClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterCatalogLifecycle) {
	sync := NewClusterCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
