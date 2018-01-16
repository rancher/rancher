package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type DynamicSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicSchema
}

type DynamicSchemaHandlerFunc func(key string, obj *DynamicSchema) error

type DynamicSchemaLister interface {
	List(namespace string, selector labels.Selector) (ret []*DynamicSchema, err error)
	Get(namespace, name string) (*DynamicSchema, error)
}

type DynamicSchemaController interface {
	Informer() cache.SharedIndexInformer
	Lister() DynamicSchemaLister
	AddHandler(name string, handler DynamicSchemaHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler DynamicSchemaHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DynamicSchemaInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*DynamicSchema) (*DynamicSchema, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*DynamicSchema, error)
	Get(name string, opts metav1.GetOptions) (*DynamicSchema, error)
	Update(*DynamicSchema) (*DynamicSchema, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DynamicSchemaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DynamicSchemaController
	AddHandler(name string, sync DynamicSchemaHandlerFunc)
	AddLifecycle(name string, lifecycle DynamicSchemaLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync DynamicSchemaHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle DynamicSchemaLifecycle)
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
		}, name)
	}
	return obj.(*DynamicSchema), nil
}

type dynamicSchemaController struct {
	controller.GenericController
}

func (c *dynamicSchemaController) Lister() DynamicSchemaLister {
	return &dynamicSchemaLister{
		controller: c,
	}
}

func (c *dynamicSchemaController) AddHandler(name string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*DynamicSchema))
	})
}

func (c *dynamicSchemaController) AddClusterScopedHandler(name, cluster string, handler DynamicSchemaHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*DynamicSchema))
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
	objectClient *clientbase.ObjectClient
	controller   DynamicSchemaController
}

func (s *dynamicSchemaClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *dynamicSchemaClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*DynamicSchema, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Update(o *DynamicSchema) (*DynamicSchema, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *dynamicSchemaClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *dynamicSchemaClient) List(opts metav1.ListOptions) (*DynamicSchemaList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DynamicSchemaList), err
}

func (s *dynamicSchemaClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *dynamicSchemaClient) Patch(o *DynamicSchema, data []byte, subresources ...string) (*DynamicSchema, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*DynamicSchema), err
}

func (s *dynamicSchemaClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *dynamicSchemaClient) AddHandler(name string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *dynamicSchemaClient) AddLifecycle(name string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedHandler(name, clusterName string, sync DynamicSchemaHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *dynamicSchemaClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle DynamicSchemaLifecycle) {
	sync := NewDynamicSchemaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
