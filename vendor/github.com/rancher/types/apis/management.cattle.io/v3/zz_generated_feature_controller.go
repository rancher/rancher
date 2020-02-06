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
	FeatureGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Feature",
	}
	FeatureResource = metav1.APIResource{
		Name:         "features",
		SingularName: "feature",
		Namespaced:   false,
		Kind:         FeatureGroupVersionKind.Kind,
	}

	FeatureGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "features",
	}
)

func init() {
	resource.Put(FeatureGroupVersionResource)
}

func NewFeature(namespace, name string, obj Feature) *Feature {
	obj.APIVersion, obj.Kind = FeatureGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type FeatureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Feature `json:"items"`
}

type FeatureHandlerFunc func(key string, obj *Feature) (runtime.Object, error)

type FeatureChangeHandlerFunc func(obj *Feature) (runtime.Object, error)

type FeatureLister interface {
	List(namespace string, selector labels.Selector) (ret []*Feature, err error)
	Get(namespace, name string) (*Feature, error)
}

type FeatureController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() FeatureLister
	AddHandler(ctx context.Context, name string, handler FeatureHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync FeatureHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler FeatureHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler FeatureHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type FeatureInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Feature) (*Feature, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Feature, error)
	Get(name string, opts metav1.GetOptions) (*Feature, error)
	Update(*Feature) (*Feature, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*FeatureList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*FeatureList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() FeatureController
	AddHandler(ctx context.Context, name string, sync FeatureHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync FeatureHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle FeatureLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle FeatureLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync FeatureHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync FeatureHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle FeatureLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle FeatureLifecycle)
}

type featureLister struct {
	controller *featureController
}

func (l *featureLister) List(namespace string, selector labels.Selector) (ret []*Feature, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Feature))
	})
	return
}

func (l *featureLister) Get(namespace, name string) (*Feature, error) {
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
			Group:    FeatureGroupVersionKind.Group,
			Resource: "feature",
		}, key)
	}
	return obj.(*Feature), nil
}

type featureController struct {
	controller.GenericController
}

func (c *featureController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *featureController) Lister() FeatureLister {
	return &featureLister{
		controller: c,
	}
}

func (c *featureController) AddHandler(ctx context.Context, name string, handler FeatureHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Feature); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *featureController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler FeatureHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Feature); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *featureController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler FeatureHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Feature); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *featureController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler FeatureHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Feature); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type featureFactory struct {
}

func (c featureFactory) Object() runtime.Object {
	return &Feature{}
}

func (c featureFactory) List() runtime.Object {
	return &FeatureList{}
}

func (s *featureClient) Controller() FeatureController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.featureControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(FeatureGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &featureController{
		GenericController: genericController,
	}

	s.client.featureControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type featureClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   FeatureController
}

func (s *featureClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *featureClient) Create(o *Feature) (*Feature, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Feature), err
}

func (s *featureClient) Get(name string, opts metav1.GetOptions) (*Feature, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Feature), err
}

func (s *featureClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Feature, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Feature), err
}

func (s *featureClient) Update(o *Feature) (*Feature, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Feature), err
}

func (s *featureClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *featureClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *featureClient) List(opts metav1.ListOptions) (*FeatureList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*FeatureList), err
}

func (s *featureClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*FeatureList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*FeatureList), err
}

func (s *featureClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *featureClient) Patch(o *Feature, patchType types.PatchType, data []byte, subresources ...string) (*Feature, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Feature), err
}

func (s *featureClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *featureClient) AddHandler(ctx context.Context, name string, sync FeatureHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *featureClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync FeatureHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *featureClient) AddLifecycle(ctx context.Context, name string, lifecycle FeatureLifecycle) {
	sync := NewFeatureLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *featureClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle FeatureLifecycle) {
	sync := NewFeatureLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *featureClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync FeatureHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *featureClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync FeatureHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *featureClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle FeatureLifecycle) {
	sync := NewFeatureLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *featureClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle FeatureLifecycle) {
	sync := NewFeatureLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
