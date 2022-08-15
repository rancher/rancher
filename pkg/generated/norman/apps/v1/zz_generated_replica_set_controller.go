package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/apps/v1"
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

	ReplicaSetGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "replicasets",
	}
)

func init() {
	resource.Put(ReplicaSetGroupVersionResource)
}

// Deprecated: use v1.ReplicaSet instead
type ReplicaSet = v1.ReplicaSet

func NewReplicaSet(namespace, name string, obj v1.ReplicaSet) *v1.ReplicaSet {
	obj.APIVersion, obj.Kind = ReplicaSetGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ReplicaSetHandlerFunc func(key string, obj *v1.ReplicaSet) (runtime.Object, error)

type ReplicaSetChangeHandlerFunc func(obj *v1.ReplicaSet) (runtime.Object, error)

type ReplicaSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ReplicaSet, err error)
	Get(namespace, name string) (*v1.ReplicaSet, error)
}

type ReplicaSetController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ReplicaSetLister
	AddHandler(ctx context.Context, name string, handler ReplicaSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicaSetHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ReplicaSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ReplicaSetHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ReplicaSetInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ReplicaSet) (*v1.ReplicaSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicaSet, error)
	Get(name string, opts metav1.GetOptions) (*v1.ReplicaSet, error)
	Update(*v1.ReplicaSet) (*v1.ReplicaSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ReplicaSetList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ReplicaSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ReplicaSetController
	AddHandler(ctx context.Context, name string, sync ReplicaSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicaSetHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ReplicaSetLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ReplicaSetLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ReplicaSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ReplicaSetHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ReplicaSetLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ReplicaSetLifecycle)
}

type replicaSetLister struct {
	ns         string
	controller *replicaSetController
}

func (l *replicaSetLister) List(namespace string, selector labels.Selector) (ret []*v1.ReplicaSet, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ReplicaSet))
	})
	return
}

func (l *replicaSetLister) Get(namespace, name string) (*v1.ReplicaSet, error) {
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
			Resource: ReplicaSetGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ReplicaSet), nil
}

type replicaSetController struct {
	ns string
	controller.GenericController
}

func (c *replicaSetController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *replicaSetController) Lister() ReplicaSetLister {
	return &replicaSetLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *replicaSetController) AddHandler(ctx context.Context, name string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicaSetController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicaSetController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *replicaSetController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ReplicaSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ReplicaSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type replicaSetFactory struct {
}

func (c replicaSetFactory) Object() runtime.Object {
	return &v1.ReplicaSet{}
}

func (c replicaSetFactory) List() runtime.Object {
	return &v1.ReplicaSetList{}
}

func (s *replicaSetClient) Controller() ReplicaSetController {
	genericController := controller.NewGenericController(s.ns, ReplicaSetGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ReplicaSetGroupVersionResource, ReplicaSetGroupVersionKind.Kind, true))

	return &replicaSetController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type replicaSetClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ReplicaSetController
}

func (s *replicaSetClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *replicaSetClient) Create(o *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) Get(name string, opts metav1.GetOptions) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) Update(o *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) UpdateStatus(o *v1.ReplicaSet) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *replicaSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *replicaSetClient) List(opts metav1.ListOptions) (*v1.ReplicaSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ReplicaSetList), err
}

func (s *replicaSetClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ReplicaSetList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ReplicaSetList), err
}

func (s *replicaSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *replicaSetClient) Patch(o *v1.ReplicaSet, patchType types.PatchType, data []byte, subresources ...string) (*v1.ReplicaSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ReplicaSet), err
}

func (s *replicaSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *replicaSetClient) AddHandler(ctx context.Context, name string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *replicaSetClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *replicaSetClient) AddLifecycle(ctx context.Context, name string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *replicaSetClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *replicaSetClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ReplicaSetHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *replicaSetClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ReplicaSetLifecycle) {
	sync := NewReplicaSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
