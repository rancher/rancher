package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	RkeK8sSystemImageGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RkeK8sSystemImage",
	}
	RkeK8sSystemImageResource = metav1.APIResource{
		Name:         "rkek8ssystemimages",
		SingularName: "rkek8ssystemimage",
		Namespaced:   true,

		Kind: RkeK8sSystemImageGroupVersionKind.Kind,
	}

	RkeK8sSystemImageGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rkek8ssystemimages",
	}
)

func init() {
	resource.Put(RkeK8sSystemImageGroupVersionResource)
}

// Deprecated: use v3.RkeK8sSystemImage instead
type RkeK8sSystemImage = v3.RkeK8sSystemImage

func NewRkeK8sSystemImage(namespace, name string, obj v3.RkeK8sSystemImage) *v3.RkeK8sSystemImage {
	obj.APIVersion, obj.Kind = RkeK8sSystemImageGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RkeK8sSystemImageHandlerFunc func(key string, obj *v3.RkeK8sSystemImage) (runtime.Object, error)

type RkeK8sSystemImageChangeHandlerFunc func(obj *v3.RkeK8sSystemImage) (runtime.Object, error)

type RkeK8sSystemImageLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.RkeK8sSystemImage, err error)
	Get(namespace, name string) (*v3.RkeK8sSystemImage, error)
}

type RkeK8sSystemImageController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RkeK8sSystemImageLister
	AddHandler(ctx context.Context, name string, handler RkeK8sSystemImageHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RkeK8sSystemImageHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RkeK8sSystemImageHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RkeK8sSystemImageHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type RkeK8sSystemImageInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RkeK8sSystemImage, error)
	Get(name string, opts metav1.GetOptions) (*v3.RkeK8sSystemImage, error)
	Update(*v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.RkeK8sSystemImageList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RkeK8sSystemImageList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RkeK8sSystemImageController
	AddHandler(ctx context.Context, name string, sync RkeK8sSystemImageHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RkeK8sSystemImageHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RkeK8sSystemImageLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RkeK8sSystemImageLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RkeK8sSystemImageHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RkeK8sSystemImageHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RkeK8sSystemImageLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RkeK8sSystemImageLifecycle)
}

type rkeK8sSystemImageLister struct {
	ns         string
	controller *rkeK8sSystemImageController
}

func (l *rkeK8sSystemImageLister) List(namespace string, selector labels.Selector) (ret []*v3.RkeK8sSystemImage, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.RkeK8sSystemImage))
	})
	return
}

func (l *rkeK8sSystemImageLister) Get(namespace, name string) (*v3.RkeK8sSystemImage, error) {
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
			Group:    RkeK8sSystemImageGroupVersionKind.Group,
			Resource: RkeK8sSystemImageGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.RkeK8sSystemImage), nil
}

type rkeK8sSystemImageController struct {
	ns string
	controller.GenericController
}

func (c *rkeK8sSystemImageController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rkeK8sSystemImageController) Lister() RkeK8sSystemImageLister {
	return &rkeK8sSystemImageLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *rkeK8sSystemImageController) AddHandler(ctx context.Context, name string, handler RkeK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sSystemImage); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sSystemImageController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RkeK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sSystemImage); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sSystemImageController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RkeK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sSystemImage); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sSystemImageController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RkeK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sSystemImage); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rkeK8sSystemImageFactory struct {
}

func (c rkeK8sSystemImageFactory) Object() runtime.Object {
	return &v3.RkeK8sSystemImage{}
}

func (c rkeK8sSystemImageFactory) List() runtime.Object {
	return &v3.RkeK8sSystemImageList{}
}

func (s *rkeK8sSystemImageClient) Controller() RkeK8sSystemImageController {
	genericController := controller.NewGenericController(s.ns, RkeK8sSystemImageGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(RkeK8sSystemImageGroupVersionResource, RkeK8sSystemImageGroupVersionKind.Kind, true))

	return &rkeK8sSystemImageController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type rkeK8sSystemImageClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RkeK8sSystemImageController
}

func (s *rkeK8sSystemImageClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rkeK8sSystemImageClient) Create(o *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.RkeK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) Get(name string, opts metav1.GetOptions) (*v3.RkeK8sSystemImage, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.RkeK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RkeK8sSystemImage, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.RkeK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) Update(o *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.RkeK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) UpdateStatus(o *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.RkeK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rkeK8sSystemImageClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rkeK8sSystemImageClient) List(opts metav1.ListOptions) (*v3.RkeK8sSystemImageList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.RkeK8sSystemImageList), err
}

func (s *rkeK8sSystemImageClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RkeK8sSystemImageList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.RkeK8sSystemImageList), err
}

func (s *rkeK8sSystemImageClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rkeK8sSystemImageClient) Patch(o *v3.RkeK8sSystemImage, patchType types.PatchType, data []byte, subresources ...string) (*v3.RkeK8sSystemImage, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.RkeK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rkeK8sSystemImageClient) AddHandler(ctx context.Context, name string, sync RkeK8sSystemImageHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sSystemImageClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RkeK8sSystemImageHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sSystemImageClient) AddLifecycle(ctx context.Context, name string, lifecycle RkeK8sSystemImageLifecycle) {
	sync := NewRkeK8sSystemImageLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sSystemImageClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RkeK8sSystemImageLifecycle) {
	sync := NewRkeK8sSystemImageLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RkeK8sSystemImageHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RkeK8sSystemImageHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RkeK8sSystemImageLifecycle) {
	sync := NewRkeK8sSystemImageLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RkeK8sSystemImageLifecycle) {
	sync := NewRkeK8sSystemImageLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
