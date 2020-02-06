package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v1 "k8s.io/api/storage/v1"
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
	StorageClassGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "StorageClass",
	}
	StorageClassResource = metav1.APIResource{
		Name:         "storageclasses",
		SingularName: "storageclass",
		Namespaced:   false,
		Kind:         StorageClassGroupVersionKind.Kind,
	}

	StorageClassGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "storageclasses",
	}
)

func init() {
	resource.Put(StorageClassGroupVersionResource)
}

func NewStorageClass(namespace, name string, obj v1.StorageClass) *v1.StorageClass {
	obj.APIVersion, obj.Kind = StorageClassGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type StorageClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.StorageClass `json:"items"`
}

type StorageClassHandlerFunc func(key string, obj *v1.StorageClass) (runtime.Object, error)

type StorageClassChangeHandlerFunc func(obj *v1.StorageClass) (runtime.Object, error)

type StorageClassLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.StorageClass, err error)
	Get(namespace, name string) (*v1.StorageClass, error)
}

type StorageClassController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() StorageClassLister
	AddHandler(ctx context.Context, name string, handler StorageClassHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync StorageClassHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler StorageClassHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler StorageClassHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type StorageClassInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.StorageClass) (*v1.StorageClass, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.StorageClass, error)
	Get(name string, opts metav1.GetOptions) (*v1.StorageClass, error)
	Update(*v1.StorageClass) (*v1.StorageClass, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*StorageClassList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*StorageClassList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() StorageClassController
	AddHandler(ctx context.Context, name string, sync StorageClassHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync StorageClassHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle StorageClassLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle StorageClassLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync StorageClassHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync StorageClassHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle StorageClassLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle StorageClassLifecycle)
}

type storageClassLister struct {
	controller *storageClassController
}

func (l *storageClassLister) List(namespace string, selector labels.Selector) (ret []*v1.StorageClass, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.StorageClass))
	})
	return
}

func (l *storageClassLister) Get(namespace, name string) (*v1.StorageClass, error) {
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
			Group:    StorageClassGroupVersionKind.Group,
			Resource: "storageClass",
		}, key)
	}
	return obj.(*v1.StorageClass), nil
}

type storageClassController struct {
	controller.GenericController
}

func (c *storageClassController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *storageClassController) Lister() StorageClassLister {
	return &storageClassLister{
		controller: c,
	}
}

func (c *storageClassController) AddHandler(ctx context.Context, name string, handler StorageClassHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StorageClass); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *storageClassController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler StorageClassHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StorageClass); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *storageClassController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler StorageClassHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StorageClass); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *storageClassController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler StorageClassHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.StorageClass); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type storageClassFactory struct {
}

func (c storageClassFactory) Object() runtime.Object {
	return &v1.StorageClass{}
}

func (c storageClassFactory) List() runtime.Object {
	return &StorageClassList{}
}

func (s *storageClassClient) Controller() StorageClassController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.storageClassControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(StorageClassGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &storageClassController{
		GenericController: genericController,
	}

	s.client.storageClassControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type storageClassClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   StorageClassController
}

func (s *storageClassClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *storageClassClient) Create(o *v1.StorageClass) (*v1.StorageClass, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.StorageClass), err
}

func (s *storageClassClient) Get(name string, opts metav1.GetOptions) (*v1.StorageClass, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.StorageClass), err
}

func (s *storageClassClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.StorageClass, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.StorageClass), err
}

func (s *storageClassClient) Update(o *v1.StorageClass) (*v1.StorageClass, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.StorageClass), err
}

func (s *storageClassClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *storageClassClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *storageClassClient) List(opts metav1.ListOptions) (*StorageClassList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*StorageClassList), err
}

func (s *storageClassClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*StorageClassList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*StorageClassList), err
}

func (s *storageClassClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *storageClassClient) Patch(o *v1.StorageClass, patchType types.PatchType, data []byte, subresources ...string) (*v1.StorageClass, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.StorageClass), err
}

func (s *storageClassClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *storageClassClient) AddHandler(ctx context.Context, name string, sync StorageClassHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *storageClassClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync StorageClassHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *storageClassClient) AddLifecycle(ctx context.Context, name string, lifecycle StorageClassLifecycle) {
	sync := NewStorageClassLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *storageClassClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle StorageClassLifecycle) {
	sync := NewStorageClassLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *storageClassClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync StorageClassHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *storageClassClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync StorageClassHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *storageClassClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle StorageClassLifecycle) {
	sync := NewStorageClassLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *storageClassClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle StorageClassLifecycle) {
	sync := NewStorageClassLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
