package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
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
	ReplicationControllerGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ReplicationController",
	}
	ReplicationControllerResource = metav1.APIResource{
		Name:         "replicationcontrollers",
		SingularName: "replicationcontroller",
		Namespaced:   true,

		Kind: ReplicationControllerGroupVersionKind.Kind,
	}

	ReplicationControllerGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "replicationcontrollers",
	}
)

func init() {
	resource.Put(ReplicationControllerGroupVersionResource)
}

// Deprecated: use v1.ReplicationController instead
type ReplicationController = v1.ReplicationController

func NewReplicationController(namespace, name string, obj v1.ReplicationController) *v1.ReplicationController {
	obj.APIVersion, obj.Kind = ReplicationControllerGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ReplicationControllerHandlerFunc func(key string, obj *v1.ReplicationController) (runtime.Object, error)

type ReplicationControllerChangeHandlerFunc func(obj *v1.ReplicationController) (runtime.Object, error)

type ReplicationControllerLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ReplicationController, err error)
	Get(namespace, name string) (*v1.ReplicationController, error)
}

type ReplicationControllerController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ReplicationControllerLister
	AddHandler(ctx context.Context, name string, handler ReplicationControllerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicationControllerHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ReplicationControllerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ReplicationControllerHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ReplicationControllerInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ReplicationController) (*v1.ReplicationController, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicationController, error)
	Get(name string, opts metav1.GetOptions) (*v1.ReplicationController, error)
	Update(*v1.ReplicationController) (*v1.ReplicationController, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ReplicationControllerList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ReplicationControllerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ReplicationControllerController
	AddHandler(ctx context.Context, name string, sync ReplicationControllerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicationControllerHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ReplicationControllerLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ReplicationControllerLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ReplicationControllerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ReplicationControllerHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ReplicationControllerLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ReplicationControllerLifecycle)
}

type replicationControllerLister struct {
	ns         string
	controller *replicationControllerController
}

func (l *replicationControllerLister) List(namespace string, selector labels.Selector) (ret []*v1.ReplicationController, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ReplicationController))
	})
	return
}

func (l *replicationControllerLister) Get(namespace, name string) (*v1.ReplicationController, error) {
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
			Group:    ReplicationControllerGroupVersionKind.Group,
			Resource: ReplicationControllerGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ReplicationController), nil
}

type replicationControllerController struct {
	ns string
	controller.GenericController
}

func (c *replicationControllerController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *replicationControllerController) Lister() ReplicationControllerLister {
	return &replicationControllerLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *replicationControllerController) AddHandler(ctx context.Context, name string, handler ReplicationControllerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicationController); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicationControllerController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ReplicationControllerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicationController); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicationControllerController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ReplicationControllerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicationController); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicationControllerController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ReplicationControllerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicationController); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type replicationControllerFactory struct {
}

func (c replicationControllerFactory) Object() runtime.Object {
	return &v1.ReplicationController{}
}

func (c replicationControllerFactory) List() runtime.Object {
	return &v1.ReplicationControllerList{}
}

func (s *replicationControllerClient) Controller() ReplicationControllerController {
	genericController := controller.NewGenericController(s.ns, ReplicationControllerGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ReplicationControllerGroupVersionResource, ReplicationControllerGroupVersionKind.Kind, true))

	return &replicationControllerController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type replicationControllerClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ReplicationControllerController
}

func (s *replicationControllerClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *replicationControllerClient) Create(o *v1.ReplicationController) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) Get(name string, opts metav1.GetOptions) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) Update(o *v1.ReplicationController) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) UpdateStatus(o *v1.ReplicationController) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *replicationControllerClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *replicationControllerClient) List(opts metav1.ListOptions) (*v1.ReplicationControllerList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ReplicationControllerList), err
}

func (s *replicationControllerClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ReplicationControllerList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ReplicationControllerList), err
}

func (s *replicationControllerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *replicationControllerClient) Patch(o *v1.ReplicationController, patchType types.PatchType, data []byte, subresources ...string) (*v1.ReplicationController, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ReplicationController), err
}

func (s *replicationControllerClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *replicationControllerClient) AddHandler(ctx context.Context, name string, sync ReplicationControllerHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *replicationControllerClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicationControllerHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *replicationControllerClient) AddLifecycle(ctx context.Context, name string, lifecycle ReplicationControllerLifecycle) {
	sync := NewReplicationControllerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *replicationControllerClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ReplicationControllerLifecycle) {
	sync := NewReplicationControllerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *replicationControllerClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ReplicationControllerHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *replicationControllerClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ReplicationControllerHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *replicationControllerClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ReplicationControllerLifecycle) {
	sync := NewReplicationControllerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *replicationControllerClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ReplicationControllerLifecycle) {
	sync := NewReplicationControllerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
