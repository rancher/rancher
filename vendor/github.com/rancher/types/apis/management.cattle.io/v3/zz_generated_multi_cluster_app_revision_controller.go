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
	MultiClusterAppRevisionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "MultiClusterAppRevision",
	}
	MultiClusterAppRevisionResource = metav1.APIResource{
		Name:         "multiclusterapprevisions",
		SingularName: "multiclusterapprevision",
		Namespaced:   true,

		Kind: MultiClusterAppRevisionGroupVersionKind.Kind,
	}

	MultiClusterAppRevisionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "multiclusterapprevisions",
	}
)

func init() {
	resource.Put(MultiClusterAppRevisionGroupVersionResource)
}

func NewMultiClusterAppRevision(namespace, name string, obj MultiClusterAppRevision) *MultiClusterAppRevision {
	obj.APIVersion, obj.Kind = MultiClusterAppRevisionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type MultiClusterAppRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterAppRevision `json:"items"`
}

type MultiClusterAppRevisionHandlerFunc func(key string, obj *MultiClusterAppRevision) (runtime.Object, error)

type MultiClusterAppRevisionChangeHandlerFunc func(obj *MultiClusterAppRevision) (runtime.Object, error)

type MultiClusterAppRevisionLister interface {
	List(namespace string, selector labels.Selector) (ret []*MultiClusterAppRevision, err error)
	Get(namespace, name string) (*MultiClusterAppRevision, error)
}

type MultiClusterAppRevisionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() MultiClusterAppRevisionLister
	AddHandler(ctx context.Context, name string, handler MultiClusterAppRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler MultiClusterAppRevisionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type MultiClusterAppRevisionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*MultiClusterAppRevision) (*MultiClusterAppRevision, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*MultiClusterAppRevision, error)
	Get(name string, opts metav1.GetOptions) (*MultiClusterAppRevision, error)
	Update(*MultiClusterAppRevision) (*MultiClusterAppRevision, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*MultiClusterAppRevisionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*MultiClusterAppRevisionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MultiClusterAppRevisionController
	AddHandler(ctx context.Context, name string, sync MultiClusterAppRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppRevisionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppRevisionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppRevisionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle)
}

type multiClusterAppRevisionLister struct {
	controller *multiClusterAppRevisionController
}

func (l *multiClusterAppRevisionLister) List(namespace string, selector labels.Selector) (ret []*MultiClusterAppRevision, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*MultiClusterAppRevision))
	})
	return
}

func (l *multiClusterAppRevisionLister) Get(namespace, name string) (*MultiClusterAppRevision, error) {
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
			Group:    MultiClusterAppRevisionGroupVersionKind.Group,
			Resource: "multiClusterAppRevision",
		}, key)
	}
	return obj.(*MultiClusterAppRevision), nil
}

type multiClusterAppRevisionController struct {
	controller.GenericController
}

func (c *multiClusterAppRevisionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *multiClusterAppRevisionController) Lister() MultiClusterAppRevisionLister {
	return &multiClusterAppRevisionLister{
		controller: c,
	}
}

func (c *multiClusterAppRevisionController) AddHandler(ctx context.Context, name string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterAppRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppRevisionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterAppRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppRevisionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterAppRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppRevisionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler MultiClusterAppRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterAppRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type multiClusterAppRevisionFactory struct {
}

func (c multiClusterAppRevisionFactory) Object() runtime.Object {
	return &MultiClusterAppRevision{}
}

func (c multiClusterAppRevisionFactory) List() runtime.Object {
	return &MultiClusterAppRevisionList{}
}

func (s *multiClusterAppRevisionClient) Controller() MultiClusterAppRevisionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.multiClusterAppRevisionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(MultiClusterAppRevisionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &multiClusterAppRevisionController{
		GenericController: genericController,
	}

	s.client.multiClusterAppRevisionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type multiClusterAppRevisionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   MultiClusterAppRevisionController
}

func (s *multiClusterAppRevisionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *multiClusterAppRevisionClient) Create(o *MultiClusterAppRevision) (*MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) Get(name string, opts metav1.GetOptions) (*MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*MultiClusterAppRevision, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) Update(o *MultiClusterAppRevision) (*MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *multiClusterAppRevisionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *multiClusterAppRevisionClient) List(opts metav1.ListOptions) (*MultiClusterAppRevisionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*MultiClusterAppRevisionList), err
}

func (s *multiClusterAppRevisionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*MultiClusterAppRevisionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*MultiClusterAppRevisionList), err
}

func (s *multiClusterAppRevisionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *multiClusterAppRevisionClient) Patch(o *MultiClusterAppRevision, patchType types.PatchType, data []byte, subresources ...string) (*MultiClusterAppRevision, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*MultiClusterAppRevision), err
}

func (s *multiClusterAppRevisionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *multiClusterAppRevisionClient) AddHandler(ctx context.Context, name string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppRevisionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppRevisionClient) AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppRevisionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppRevisionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppRevisionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppRevisionLifecycle) {
	sync := NewMultiClusterAppRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
