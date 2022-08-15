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

	StatefulSetGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "statefulsets",
	}
)

func init() {
	resource.Put(StatefulSetGroupVersionResource)
}

// Deprecated: use v1.StatefulSet instead
type StatefulSet = v1.StatefulSet

func NewStatefulSet(namespace, name string, obj v1.StatefulSet) *v1.StatefulSet {
	obj.APIVersion, obj.Kind = StatefulSetGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type StatefulSetHandlerFunc func(key string, obj *v1.StatefulSet) (runtime.Object, error)

type StatefulSetChangeHandlerFunc func(obj *v1.StatefulSet) (runtime.Object, error)

type StatefulSetLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.StatefulSet, err error)
	Get(namespace, name string) (*v1.StatefulSet, error)
}

type StatefulSetController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() StatefulSetLister
	AddHandler(ctx context.Context, name string, handler StatefulSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync StatefulSetHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler StatefulSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler StatefulSetHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type StatefulSetInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.StatefulSet) (*v1.StatefulSet, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.StatefulSet, error)
	Get(name string, opts metav1.GetOptions) (*v1.StatefulSet, error)
	Update(*v1.StatefulSet) (*v1.StatefulSet, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.StatefulSetList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.StatefulSetList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() StatefulSetController
	AddHandler(ctx context.Context, name string, sync StatefulSetHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync StatefulSetHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle StatefulSetLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle StatefulSetLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync StatefulSetHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync StatefulSetHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle StatefulSetLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle StatefulSetLifecycle)
}

type statefulSetLister struct {
	ns         string
	controller *statefulSetController
}

func (l *statefulSetLister) List(namespace string, selector labels.Selector) (ret []*v1.StatefulSet, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.StatefulSet))
	})
	return
}

func (l *statefulSetLister) Get(namespace, name string) (*v1.StatefulSet, error) {
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
			Resource: StatefulSetGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.StatefulSet), nil
}

type statefulSetController struct {
	ns string
	controller.GenericController
}

func (c *statefulSetController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *statefulSetController) Lister() StatefulSetLister {
	return &statefulSetLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *statefulSetController) AddHandler(ctx context.Context, name string, handler StatefulSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StatefulSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *statefulSetController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler StatefulSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StatefulSet); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *statefulSetController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler StatefulSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StatefulSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *statefulSetController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler StatefulSetHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StatefulSet); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type statefulSetFactory struct {
}

func (c statefulSetFactory) Object() runtime.Object {
	return &v1.StatefulSet{}
}

func (c statefulSetFactory) List() runtime.Object {
	return &v1.StatefulSetList{}
}

func (s *statefulSetClient) Controller() StatefulSetController {
	genericController := controller.NewGenericController(s.ns, StatefulSetGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(StatefulSetGroupVersionResource, StatefulSetGroupVersionKind.Kind, true))

	return &statefulSetController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type statefulSetClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   StatefulSetController
}

func (s *statefulSetClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *statefulSetClient) Create(o *v1.StatefulSet) (*v1.StatefulSet, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.StatefulSet), err
}

func (s *statefulSetClient) Get(name string, opts metav1.GetOptions) (*v1.StatefulSet, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.StatefulSet), err
}

func (s *statefulSetClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.StatefulSet, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.StatefulSet), err
}

func (s *statefulSetClient) Update(o *v1.StatefulSet) (*v1.StatefulSet, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.StatefulSet), err
}

func (s *statefulSetClient) UpdateStatus(o *v1.StatefulSet) (*v1.StatefulSet, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.StatefulSet), err
}

func (s *statefulSetClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *statefulSetClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *statefulSetClient) List(opts metav1.ListOptions) (*v1.StatefulSetList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.StatefulSetList), err
}

func (s *statefulSetClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.StatefulSetList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.StatefulSetList), err
}

func (s *statefulSetClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *statefulSetClient) Patch(o *v1.StatefulSet, patchType types.PatchType, data []byte, subresources ...string) (*v1.StatefulSet, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.StatefulSet), err
}

func (s *statefulSetClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *statefulSetClient) AddHandler(ctx context.Context, name string, sync StatefulSetHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *statefulSetClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync StatefulSetHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *statefulSetClient) AddLifecycle(ctx context.Context, name string, lifecycle StatefulSetLifecycle) {
	sync := NewStatefulSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *statefulSetClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle StatefulSetLifecycle) {
	sync := NewStatefulSetLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *statefulSetClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync StatefulSetHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *statefulSetClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync StatefulSetHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *statefulSetClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle StatefulSetLifecycle) {
	sync := NewStatefulSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *statefulSetClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle StatefulSetLifecycle) {
	sync := NewStatefulSetLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
