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
	MultiClusterAppGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "MultiClusterApp",
	}
	MultiClusterAppResource = metav1.APIResource{
		Name:         "multiclusterapps",
		SingularName: "multiclusterapp",
		Namespaced:   true,

		Kind: MultiClusterAppGroupVersionKind.Kind,
	}

	MultiClusterAppGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "multiclusterapps",
	}
)

func init() {
	resource.Put(MultiClusterAppGroupVersionResource)
}

func NewMultiClusterApp(namespace, name string, obj MultiClusterApp) *MultiClusterApp {
	obj.APIVersion, obj.Kind = MultiClusterAppGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type MultiClusterAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterApp `json:"items"`
}

type MultiClusterAppHandlerFunc func(key string, obj *MultiClusterApp) (runtime.Object, error)

type MultiClusterAppChangeHandlerFunc func(obj *MultiClusterApp) (runtime.Object, error)

type MultiClusterAppLister interface {
	List(namespace string, selector labels.Selector) (ret []*MultiClusterApp, err error)
	Get(namespace, name string) (*MultiClusterApp, error)
}

type MultiClusterAppController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() MultiClusterAppLister
	AddHandler(ctx context.Context, name string, handler MultiClusterAppHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler MultiClusterAppHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler MultiClusterAppHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type MultiClusterAppInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*MultiClusterApp) (*MultiClusterApp, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*MultiClusterApp, error)
	Get(name string, opts metav1.GetOptions) (*MultiClusterApp, error)
	Update(*MultiClusterApp) (*MultiClusterApp, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*MultiClusterAppList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*MultiClusterAppList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MultiClusterAppController
	AddHandler(ctx context.Context, name string, sync MultiClusterAppHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppLifecycle)
}

type multiClusterAppLister struct {
	controller *multiClusterAppController
}

func (l *multiClusterAppLister) List(namespace string, selector labels.Selector) (ret []*MultiClusterApp, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*MultiClusterApp))
	})
	return
}

func (l *multiClusterAppLister) Get(namespace, name string) (*MultiClusterApp, error) {
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
			Group:    MultiClusterAppGroupVersionKind.Group,
			Resource: "multiClusterApp",
		}, key)
	}
	return obj.(*MultiClusterApp), nil
}

type multiClusterAppController struct {
	controller.GenericController
}

func (c *multiClusterAppController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *multiClusterAppController) Lister() MultiClusterAppLister {
	return &multiClusterAppLister{
		controller: c,
	}
}

func (c *multiClusterAppController) AddHandler(ctx context.Context, name string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterApp); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterApp); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterApp); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *multiClusterAppController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler MultiClusterAppHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*MultiClusterApp); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type multiClusterAppFactory struct {
}

func (c multiClusterAppFactory) Object() runtime.Object {
	return &MultiClusterApp{}
}

func (c multiClusterAppFactory) List() runtime.Object {
	return &MultiClusterAppList{}
}

func (s *multiClusterAppClient) Controller() MultiClusterAppController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.multiClusterAppControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(MultiClusterAppGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &multiClusterAppController{
		GenericController: genericController,
	}

	s.client.multiClusterAppControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type multiClusterAppClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   MultiClusterAppController
}

func (s *multiClusterAppClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *multiClusterAppClient) Create(o *MultiClusterApp) (*MultiClusterApp, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*MultiClusterApp), err
}

func (s *multiClusterAppClient) Get(name string, opts metav1.GetOptions) (*MultiClusterApp, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*MultiClusterApp), err
}

func (s *multiClusterAppClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*MultiClusterApp, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*MultiClusterApp), err
}

func (s *multiClusterAppClient) Update(o *MultiClusterApp) (*MultiClusterApp, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*MultiClusterApp), err
}

func (s *multiClusterAppClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *multiClusterAppClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *multiClusterAppClient) List(opts metav1.ListOptions) (*MultiClusterAppList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*MultiClusterAppList), err
}

func (s *multiClusterAppClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*MultiClusterAppList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*MultiClusterAppList), err
}

func (s *multiClusterAppClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *multiClusterAppClient) Patch(o *MultiClusterApp, patchType types.PatchType, data []byte, subresources ...string) (*MultiClusterApp, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*MultiClusterApp), err
}

func (s *multiClusterAppClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *multiClusterAppClient) AddHandler(ctx context.Context, name string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppClient) AddLifecycle(ctx context.Context, name string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *multiClusterAppClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *multiClusterAppClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync MultiClusterAppHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *multiClusterAppClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *multiClusterAppClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle MultiClusterAppLifecycle) {
	sync := NewMultiClusterAppLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
