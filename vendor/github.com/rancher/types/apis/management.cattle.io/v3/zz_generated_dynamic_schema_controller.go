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
	DynamicSchemaGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DynamicSchema",
	}
	DynamicSchemaResource = metav1.APIResource{
		Name:         "dynamicschemas",
		SingularName: "dynamicschema",
		Namespaced:   false,
		Kind:         DynamicSchemaGroupVersionKind.Kind,
	}

	DynamicSchemaGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "dynamicschemas",
	}
)

func init() {
	resource.Put(DynamicSchemaGroupVersionResource)
}

func NewDynamicSchema(namespace, name string, obj DynamicSchema) *DynamicSchema {
	obj.APIVersion, obj.Kind = DynamicSchemaGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DynamicSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicSchema `json:"items"`
}

type DynamicSchemaHandlerFunc func(key string, obj *DynamicSchema) (runtime.Object, error)

type DynamicSchemaChangeHandlerFunc func(obj *DynamicSchema) (runtime.Object, error)

type DynamicSchemaLister interface {
	List(namespace string, selector labels.Selector) (ret []*DynamicSchema, err error)
	Get(namespace, name string) (*DynamicSchema, error)
}

type DynamicSchemaController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DynamicSchemaLister
	AddHandler(ctx context.Context, name string, handler DynamicSchemaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DynamicSchemaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler DynamicSchemaHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DynamicSchemaInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*DynamicSchema) (*DynamicSchema, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*DynamicSchema, error)
	Get(name string, opts metav1.GetOptions) (*DynamicSchema, error)
	Update(*DynamicSchema) (*DynamicSchema, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DynamicSchemaList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*DynamicSchemaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DynamicSchemaController
	AddHandler(ctx context.Context, name string, sync DynamicSchemaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DynamicSchemaHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DynamicSchemaLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DynamicSchemaLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DynamicSchemaLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DynamicSchemaLifecycle)
}

type dynamicSchemaLister struct {
	controller *dynamicSchemaController
}

func (l *dynamicSchemaLister) List(namespace string, selector labels.Selector) (ret []*DynamicSchema, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*DynamicSchema))
	})
	return
}

func (l *dynamicSchemaLister) Get(namespace, name string) (*DynamicSchema, error) {
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
			Group:    DynamicSchemaGroupVersionKind.Group,
			Resource: "dynamicSchema",
		}, key)
	}
	return obj.(*DynamicSchema), nil
}

type dynamicSchemaController struct {
	controller.GenericController
}

func (c *dynamicSchemaController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *dynamicSchemaController) Lister() DynamicSchemaLister {
	return &dynamicSchemaLister{
		controller: c,
	}
}

func (c *dynamicSchemaController) AddHandler(ctx context.Context, name string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dynamicSchemaController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dynamicSchemaController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dynamicSchemaController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*DynamicSchema); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type dynamicSchemaFactory struct {
}

func (c dynamicSchemaFactory) Object() runtime.Object {
	return &DynamicSchema{}
}

func (c dynamicSchemaFactory) List() runtime.Object {
	return &DynamicSchemaList{}
}

func (s *dynamicSchemaClient) Controller() DynamicSchemaController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.dynamicSchemaControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DynamicSchemaGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &dynamicSchemaController{
		GenericController: genericController,
	}

	s.client.dynamicSchemaControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type dynamicSchemaClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DynamicSchemaController
}

func (s *dynamicSchemaClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *dynamicSchemaClient) Create(o *DynamicSchema) (*DynamicSchema, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Get(name string, opts metav1.GetOptions) (*DynamicSchema, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*DynamicSchema, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Update(o *DynamicSchema) (*DynamicSchema, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *dynamicSchemaClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *dynamicSchemaClient) List(opts metav1.ListOptions) (*DynamicSchemaList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DynamicSchemaList), err
}

func (s *dynamicSchemaClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*DynamicSchemaList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*DynamicSchemaList), err
}

func (s *dynamicSchemaClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *dynamicSchemaClient) Patch(o *DynamicSchema, patchType types.PatchType, data []byte, subresources ...string) (*DynamicSchema, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *dynamicSchemaClient) AddHandler(ctx context.Context, name string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *dynamicSchemaClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *dynamicSchemaClient) AddLifecycle(ctx context.Context, name string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *dynamicSchemaClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
