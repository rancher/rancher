package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	ClusterGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Cluster",
	}
	ClusterResource = metav1.APIResource{
		Name:         "clusters",
		SingularName: "cluster",
		Namespaced:   false,
		Kind:         ClusterGroupVersionKind.Kind,
	}

	ClusterGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusters",
	}
)

func init() {
	resource.Put(ClusterGroupVersionResource)
}

// Deprecated: use v3.Cluster instead
type Cluster = v3.Cluster

func NewCluster(namespace, name string, obj v3.Cluster) *v3.Cluster {
	obj.APIVersion, obj.Kind = ClusterGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterHandlerFunc func(key string, obj *v3.Cluster) (runtime.Object, error)

type ClusterChangeHandlerFunc func(obj *v3.Cluster) (runtime.Object, error)

type ClusterLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Cluster, err error)
	Get(namespace, name string) (*v3.Cluster, error)
}

type ClusterController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterLister
	AddHandler(ctx context.Context, name string, handler ClusterHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Cluster) (*v3.Cluster, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Cluster, error)
	Get(name string, opts metav1.GetOptions) (*v3.Cluster, error)
	Update(*v3.Cluster) (*v3.Cluster, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ClusterList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterController
	AddHandler(ctx context.Context, name string, sync ClusterHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterLifecycle)
}

type clusterLister struct {
	ns         string
	controller *clusterController
}

func (l *clusterLister) List(namespace string, selector labels.Selector) (ret []*v3.Cluster, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Cluster))
	})
	return
}

func (l *clusterLister) Get(namespace, name string) (*v3.Cluster, error) {
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
			Group:    ClusterGroupVersionKind.Group,
			Resource: ClusterGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Cluster), nil
}

type clusterController struct {
	ns string
	controller.GenericController
}

func (c *clusterController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterController) Lister() ClusterLister {
	return &clusterLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterController) AddHandler(ctx context.Context, name string, handler ClusterHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Cluster); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Cluster); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Cluster); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Cluster); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterFactory struct {
}

func (c clusterFactory) Object() runtime.Object {
	return &v3.Cluster{}
}

func (c clusterFactory) List() runtime.Object {
	return &v3.ClusterList{}
}

func (s *clusterClient) Controller() ClusterController {
	genericController := controller.NewGenericController(s.ns, ClusterGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterGroupVersionResource, ClusterGroupVersionKind.Kind, false))

	return &clusterController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterController
}

func (s *clusterClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterClient) Create(o *v3.Cluster) (*v3.Cluster, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Cluster), err
}

func (s *clusterClient) Get(name string, opts metav1.GetOptions) (*v3.Cluster, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Cluster), err
}

func (s *clusterClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Cluster, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Cluster), err
}

func (s *clusterClient) Update(o *v3.Cluster) (*v3.Cluster, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Cluster), err
}

func (s *clusterClient) UpdateStatus(o *v3.Cluster) (*v3.Cluster, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Cluster), err
}

func (s *clusterClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterClient) List(opts metav1.ListOptions) (*v3.ClusterList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ClusterList), err
}

func (s *clusterClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ClusterList), err
}

func (s *clusterClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterClient) Patch(o *v3.Cluster, patchType types.PatchType, data []byte, subresources ...string) (*v3.Cluster, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Cluster), err
}

func (s *clusterClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterClient) AddHandler(ctx context.Context, name string, sync ClusterHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterLifecycle) {
	sync := NewClusterLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterLifecycle) {
	sync := NewClusterLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterLifecycle) {
	sync := NewClusterLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterLifecycle) {
	sync := NewClusterLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
