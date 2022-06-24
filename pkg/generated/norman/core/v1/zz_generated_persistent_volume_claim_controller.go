package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
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
	PersistentVolumeClaimGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PersistentVolumeClaim",
	}
	PersistentVolumeClaimResource = metav1.APIResource{
		Name:         "persistentvolumeclaims",
		SingularName: "persistentvolumeclaim",
		Namespaced:   true,

		Kind: PersistentVolumeClaimGroupVersionKind.Kind,
	}

	PersistentVolumeClaimGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "persistentvolumeclaims",
	}
)

func init() {
	resource.Put(PersistentVolumeClaimGroupVersionResource)
}

// Deprecated: use v1.PersistentVolumeClaim instead
type PersistentVolumeClaim = v1.PersistentVolumeClaim

func NewPersistentVolumeClaim(namespace, name string, obj v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	obj.APIVersion, obj.Kind = PersistentVolumeClaimGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PersistentVolumeClaimHandlerFunc func(key string, obj *v1.PersistentVolumeClaim) (runtime.Object, error)

type PersistentVolumeClaimChangeHandlerFunc func(obj *v1.PersistentVolumeClaim) (runtime.Object, error)

type PersistentVolumeClaimLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.PersistentVolumeClaim, err error)
	Get(namespace, name string) (*v1.PersistentVolumeClaim, error)
}

type PersistentVolumeClaimController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PersistentVolumeClaimLister
	AddHandler(ctx context.Context, name string, handler PersistentVolumeClaimHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PersistentVolumeClaimHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PersistentVolumeClaimHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PersistentVolumeClaimHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PersistentVolumeClaimInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error)
	Get(name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error)
	Update(*v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.PersistentVolumeClaimList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.PersistentVolumeClaimList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PersistentVolumeClaimController
	AddHandler(ctx context.Context, name string, sync PersistentVolumeClaimHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PersistentVolumeClaimHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PersistentVolumeClaimLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PersistentVolumeClaimLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PersistentVolumeClaimHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PersistentVolumeClaimHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PersistentVolumeClaimLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PersistentVolumeClaimLifecycle)
}

type persistentVolumeClaimLister struct {
	ns         string
	controller *persistentVolumeClaimController
}

func (l *persistentVolumeClaimLister) List(namespace string, selector labels.Selector) (ret []*v1.PersistentVolumeClaim, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.PersistentVolumeClaim))
	})
	return
}

func (l *persistentVolumeClaimLister) Get(namespace, name string) (*v1.PersistentVolumeClaim, error) {
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
			Group:    PersistentVolumeClaimGroupVersionKind.Group,
			Resource: PersistentVolumeClaimGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.PersistentVolumeClaim), nil
}

type persistentVolumeClaimController struct {
	ns string
	controller.GenericController
}

func (c *persistentVolumeClaimController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *persistentVolumeClaimController) Lister() PersistentVolumeClaimLister {
	return &persistentVolumeClaimLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *persistentVolumeClaimController) AddHandler(ctx context.Context, name string, handler PersistentVolumeClaimHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.PersistentVolumeClaim); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *persistentVolumeClaimController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PersistentVolumeClaimHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.PersistentVolumeClaim); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *persistentVolumeClaimController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PersistentVolumeClaimHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.PersistentVolumeClaim); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *persistentVolumeClaimController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PersistentVolumeClaimHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.PersistentVolumeClaim); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type persistentVolumeClaimFactory struct {
}

func (c persistentVolumeClaimFactory) Object() runtime.Object {
	return &v1.PersistentVolumeClaim{}
}

func (c persistentVolumeClaimFactory) List() runtime.Object {
	return &v1.PersistentVolumeClaimList{}
}

func (s *persistentVolumeClaimClient) Controller() PersistentVolumeClaimController {
	genericController := controller.NewGenericController(s.ns, PersistentVolumeClaimGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PersistentVolumeClaimGroupVersionResource, PersistentVolumeClaimGroupVersionKind.Kind, true))

	return &persistentVolumeClaimController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type persistentVolumeClaimClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PersistentVolumeClaimController
}

func (s *persistentVolumeClaimClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *persistentVolumeClaimClient) Create(o *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) Get(name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) Update(o *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) UpdateStatus(o *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *persistentVolumeClaimClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *persistentVolumeClaimClient) List(opts metav1.ListOptions) (*v1.PersistentVolumeClaimList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.PersistentVolumeClaimList), err
}

func (s *persistentVolumeClaimClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.PersistentVolumeClaimList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.PersistentVolumeClaimList), err
}

func (s *persistentVolumeClaimClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *persistentVolumeClaimClient) Patch(o *v1.PersistentVolumeClaim, patchType types.PatchType, data []byte, subresources ...string) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *persistentVolumeClaimClient) AddHandler(ctx context.Context, name string, sync PersistentVolumeClaimHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *persistentVolumeClaimClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PersistentVolumeClaimHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *persistentVolumeClaimClient) AddLifecycle(ctx context.Context, name string, lifecycle PersistentVolumeClaimLifecycle) {
	sync := NewPersistentVolumeClaimLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *persistentVolumeClaimClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PersistentVolumeClaimLifecycle) {
	sync := NewPersistentVolumeClaimLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *persistentVolumeClaimClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PersistentVolumeClaimHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *persistentVolumeClaimClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PersistentVolumeClaimHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *persistentVolumeClaimClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PersistentVolumeClaimLifecycle) {
	sync := NewPersistentVolumeClaimLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *persistentVolumeClaimClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PersistentVolumeClaimLifecycle) {
	sync := NewPersistentVolumeClaimLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
