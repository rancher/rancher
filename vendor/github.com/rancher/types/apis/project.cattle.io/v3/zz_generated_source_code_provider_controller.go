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
	SourceCodeProviderGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeProvider",
	}
	SourceCodeProviderResource = metav1.APIResource{
		Name:         "sourcecodeproviders",
		SingularName: "sourcecodeprovider",
		Namespaced:   false,
		Kind:         SourceCodeProviderGroupVersionKind.Kind,
	}

	SourceCodeProviderGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "sourcecodeproviders",
	}
)

func init() {
	resource.Put(SourceCodeProviderGroupVersionResource)
}

func NewSourceCodeProvider(namespace, name string, obj SourceCodeProvider) *SourceCodeProvider {
	obj.APIVersion, obj.Kind = SourceCodeProviderGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeProvider `json:"items"`
}

type SourceCodeProviderHandlerFunc func(key string, obj *SourceCodeProvider) (runtime.Object, error)

type SourceCodeProviderChangeHandlerFunc func(obj *SourceCodeProvider) (runtime.Object, error)

type SourceCodeProviderLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeProvider, err error)
	Get(namespace, name string) (*SourceCodeProvider, error)
}

type SourceCodeProviderController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeProviderLister
	AddHandler(ctx context.Context, name string, handler SourceCodeProviderHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeProviderHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SourceCodeProviderHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler SourceCodeProviderHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeProviderInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeProvider) (*SourceCodeProvider, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Update(*SourceCodeProvider) (*SourceCodeProvider, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeProviderList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*SourceCodeProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeProviderController
	AddHandler(ctx context.Context, name string, sync SourceCodeProviderHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeProviderHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeProviderLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SourceCodeProviderLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeProviderHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SourceCodeProviderHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeProviderLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SourceCodeProviderLifecycle)
}

type sourceCodeProviderLister struct {
	controller *sourceCodeProviderController
}

func (l *sourceCodeProviderLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeProvider, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeProvider))
	})
	return
}

func (l *sourceCodeProviderLister) Get(namespace, name string) (*SourceCodeProvider, error) {
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
			Group:    SourceCodeProviderGroupVersionKind.Group,
			Resource: "sourceCodeProvider",
		}, key)
	}
	return obj.(*SourceCodeProvider), nil
}

type sourceCodeProviderController struct {
	controller.GenericController
}

func (c *sourceCodeProviderController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeProviderController) Lister() SourceCodeProviderLister {
	return &sourceCodeProviderLister{
		controller: c,
	}
}

func (c *sourceCodeProviderController) AddHandler(ctx context.Context, name string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProvider); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeProviderController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeProvider); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeProviderFactory struct {
}

func (c sourceCodeProviderFactory) Object() runtime.Object {
	return &SourceCodeProvider{}
}

func (c sourceCodeProviderFactory) List() runtime.Object {
	return &SourceCodeProviderList{}
}

func (s *sourceCodeProviderClient) Controller() SourceCodeProviderController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeProviderControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeProviderGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeProviderController{
		GenericController: genericController,
	}

	s.client.sourceCodeProviderControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeProviderClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeProviderController
}

func (s *sourceCodeProviderClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeProviderClient) Create(o *SourceCodeProvider) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Get(name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Update(o *SourceCodeProvider) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeProviderClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeProviderClient) List(opts metav1.ListOptions) (*SourceCodeProviderList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeProviderList), err
}

func (s *sourceCodeProviderClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*SourceCodeProviderList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*SourceCodeProviderList), err
}

func (s *sourceCodeProviderClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeProviderClient) Patch(o *SourceCodeProvider, patchType types.PatchType, data []byte, subresources ...string) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeProviderClient) AddHandler(ctx context.Context, name string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeProviderClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sourceCodeProviderClient) AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeProviderClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
