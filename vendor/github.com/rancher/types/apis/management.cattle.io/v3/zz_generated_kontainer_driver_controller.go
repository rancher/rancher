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
	KontainerDriverGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "KontainerDriver",
	}
	KontainerDriverResource = metav1.APIResource{
		Name:         "kontainerdrivers",
		SingularName: "kontainerdriver",
		Namespaced:   false,
		Kind:         KontainerDriverGroupVersionKind.Kind,
	}

	KontainerDriverGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "kontainerdrivers",
	}
)

func init() {
	resource.Put(KontainerDriverGroupVersionResource)
}

func NewKontainerDriver(namespace, name string, obj KontainerDriver) *KontainerDriver {
	obj.APIVersion, obj.Kind = KontainerDriverGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type KontainerDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KontainerDriver `json:"items"`
}

type KontainerDriverHandlerFunc func(key string, obj *KontainerDriver) (runtime.Object, error)

type KontainerDriverChangeHandlerFunc func(obj *KontainerDriver) (runtime.Object, error)

type KontainerDriverLister interface {
	List(namespace string, selector labels.Selector) (ret []*KontainerDriver, err error)
	Get(namespace, name string) (*KontainerDriver, error)
}

type KontainerDriverController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() KontainerDriverLister
	AddHandler(ctx context.Context, name string, handler KontainerDriverHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync KontainerDriverHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler KontainerDriverHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler KontainerDriverHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type KontainerDriverInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*KontainerDriver) (*KontainerDriver, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*KontainerDriver, error)
	Get(name string, opts metav1.GetOptions) (*KontainerDriver, error)
	Update(*KontainerDriver) (*KontainerDriver, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*KontainerDriverList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*KontainerDriverList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() KontainerDriverController
	AddHandler(ctx context.Context, name string, sync KontainerDriverHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync KontainerDriverHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle KontainerDriverLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle KontainerDriverLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync KontainerDriverHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync KontainerDriverHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle KontainerDriverLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle KontainerDriverLifecycle)
}

type kontainerDriverLister struct {
	controller *kontainerDriverController
}

func (l *kontainerDriverLister) List(namespace string, selector labels.Selector) (ret []*KontainerDriver, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*KontainerDriver))
	})
	return
}

func (l *kontainerDriverLister) Get(namespace, name string) (*KontainerDriver, error) {
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
			Group:    KontainerDriverGroupVersionKind.Group,
			Resource: "kontainerDriver",
		}, key)
	}
	return obj.(*KontainerDriver), nil
}

type kontainerDriverController struct {
	controller.GenericController
}

func (c *kontainerDriverController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *kontainerDriverController) Lister() KontainerDriverLister {
	return &kontainerDriverLister{
		controller: c,
	}
}

func (c *kontainerDriverController) AddHandler(ctx context.Context, name string, handler KontainerDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*KontainerDriver); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *kontainerDriverController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler KontainerDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*KontainerDriver); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *kontainerDriverController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler KontainerDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*KontainerDriver); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *kontainerDriverController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler KontainerDriverHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*KontainerDriver); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type kontainerDriverFactory struct {
}

func (c kontainerDriverFactory) Object() runtime.Object {
	return &KontainerDriver{}
}

func (c kontainerDriverFactory) List() runtime.Object {
	return &KontainerDriverList{}
}

func (s *kontainerDriverClient) Controller() KontainerDriverController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.kontainerDriverControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(KontainerDriverGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &kontainerDriverController{
		GenericController: genericController,
	}

	s.client.kontainerDriverControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type kontainerDriverClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   KontainerDriverController
}

func (s *kontainerDriverClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *kontainerDriverClient) Create(o *KontainerDriver) (*KontainerDriver, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*KontainerDriver), err
}

func (s *kontainerDriverClient) Get(name string, opts metav1.GetOptions) (*KontainerDriver, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*KontainerDriver), err
}

func (s *kontainerDriverClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*KontainerDriver, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*KontainerDriver), err
}

func (s *kontainerDriverClient) Update(o *KontainerDriver) (*KontainerDriver, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*KontainerDriver), err
}

func (s *kontainerDriverClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *kontainerDriverClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *kontainerDriverClient) List(opts metav1.ListOptions) (*KontainerDriverList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*KontainerDriverList), err
}

func (s *kontainerDriverClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*KontainerDriverList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*KontainerDriverList), err
}

func (s *kontainerDriverClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *kontainerDriverClient) Patch(o *KontainerDriver, patchType types.PatchType, data []byte, subresources ...string) (*KontainerDriver, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*KontainerDriver), err
}

func (s *kontainerDriverClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *kontainerDriverClient) AddHandler(ctx context.Context, name string, sync KontainerDriverHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *kontainerDriverClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync KontainerDriverHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *kontainerDriverClient) AddLifecycle(ctx context.Context, name string, lifecycle KontainerDriverLifecycle) {
	sync := NewKontainerDriverLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *kontainerDriverClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle KontainerDriverLifecycle) {
	sync := NewKontainerDriverLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *kontainerDriverClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync KontainerDriverHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *kontainerDriverClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync KontainerDriverHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *kontainerDriverClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle KontainerDriverLifecycle) {
	sync := NewKontainerDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *kontainerDriverClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle KontainerDriverLifecycle) {
	sync := NewKontainerDriverLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
