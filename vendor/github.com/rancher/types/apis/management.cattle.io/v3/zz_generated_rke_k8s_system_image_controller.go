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
	RKEK8sSystemImageGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RKEK8sSystemImage",
	}
	RKEK8sSystemImageResource = metav1.APIResource{
		Name:         "rkek8ssystemimages",
		SingularName: "rkek8ssystemimage",
		Namespaced:   true,

		Kind: RKEK8sSystemImageGroupVersionKind.Kind,
	}

	RKEK8sSystemImageGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rkek8ssystemimages",
	}
)

func init() {
	resource.Put(RKEK8sSystemImageGroupVersionResource)
}

func NewRKEK8sSystemImage(namespace, name string, obj RKEK8sSystemImage) *RKEK8sSystemImage {
	obj.APIVersion, obj.Kind = RKEK8sSystemImageGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RKEK8sSystemImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RKEK8sSystemImage `json:"items"`
}

type RKEK8sSystemImageHandlerFunc func(key string, obj *RKEK8sSystemImage) (runtime.Object, error)

type RKEK8sSystemImageChangeHandlerFunc func(obj *RKEK8sSystemImage) (runtime.Object, error)

type RKEK8sSystemImageLister interface {
	List(namespace string, selector labels.Selector) (ret []*RKEK8sSystemImage, err error)
	Get(namespace, name string) (*RKEK8sSystemImage, error)
}

type RKEK8sSystemImageController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RKEK8sSystemImageLister
	AddHandler(ctx context.Context, name string, handler RKEK8sSystemImageHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sSystemImageHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RKEK8sSystemImageHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RKEK8sSystemImageHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RKEK8sSystemImageInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*RKEK8sSystemImage) (*RKEK8sSystemImage, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEK8sSystemImage, error)
	Get(name string, opts metav1.GetOptions) (*RKEK8sSystemImage, error)
	Update(*RKEK8sSystemImage) (*RKEK8sSystemImage, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RKEK8sSystemImageList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*RKEK8sSystemImageList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RKEK8sSystemImageController
	AddHandler(ctx context.Context, name string, sync RKEK8sSystemImageHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sSystemImageHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RKEK8sSystemImageLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEK8sSystemImageLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEK8sSystemImageHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEK8sSystemImageHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEK8sSystemImageLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEK8sSystemImageLifecycle)
}

type rkeK8sSystemImageLister struct {
	controller *rkeK8sSystemImageController
}

func (l *rkeK8sSystemImageLister) List(namespace string, selector labels.Selector) (ret []*RKEK8sSystemImage, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*RKEK8sSystemImage))
	})
	return
}

func (l *rkeK8sSystemImageLister) Get(namespace, name string) (*RKEK8sSystemImage, error) {
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
			Group:    RKEK8sSystemImageGroupVersionKind.Group,
			Resource: "rkeK8sSystemImage",
		}, key)
	}
	return obj.(*RKEK8sSystemImage), nil
}

type rkeK8sSystemImageController struct {
	controller.GenericController
}

func (c *rkeK8sSystemImageController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rkeK8sSystemImageController) Lister() RKEK8sSystemImageLister {
	return &rkeK8sSystemImageLister{
		controller: c,
	}
}

func (c *rkeK8sSystemImageController) AddHandler(ctx context.Context, name string, handler RKEK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sSystemImage); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sSystemImageController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RKEK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sSystemImage); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sSystemImageController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RKEK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sSystemImage); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sSystemImageController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RKEK8sSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sSystemImage); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rkeK8sSystemImageFactory struct {
}

func (c rkeK8sSystemImageFactory) Object() runtime.Object {
	return &RKEK8sSystemImage{}
}

func (c rkeK8sSystemImageFactory) List() runtime.Object {
	return &RKEK8sSystemImageList{}
}

func (s *rkeK8sSystemImageClient) Controller() RKEK8sSystemImageController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.rkeK8sSystemImageControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RKEK8sSystemImageGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &rkeK8sSystemImageController{
		GenericController: genericController,
	}

	s.client.rkeK8sSystemImageControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type rkeK8sSystemImageClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RKEK8sSystemImageController
}

func (s *rkeK8sSystemImageClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rkeK8sSystemImageClient) Create(o *RKEK8sSystemImage) (*RKEK8sSystemImage, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*RKEK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) Get(name string, opts metav1.GetOptions) (*RKEK8sSystemImage, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*RKEK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEK8sSystemImage, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*RKEK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) Update(o *RKEK8sSystemImage) (*RKEK8sSystemImage, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*RKEK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rkeK8sSystemImageClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rkeK8sSystemImageClient) List(opts metav1.ListOptions) (*RKEK8sSystemImageList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RKEK8sSystemImageList), err
}

func (s *rkeK8sSystemImageClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*RKEK8sSystemImageList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*RKEK8sSystemImageList), err
}

func (s *rkeK8sSystemImageClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rkeK8sSystemImageClient) Patch(o *RKEK8sSystemImage, patchType types.PatchType, data []byte, subresources ...string) (*RKEK8sSystemImage, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*RKEK8sSystemImage), err
}

func (s *rkeK8sSystemImageClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rkeK8sSystemImageClient) AddHandler(ctx context.Context, name string, sync RKEK8sSystemImageHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sSystemImageClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sSystemImageHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sSystemImageClient) AddLifecycle(ctx context.Context, name string, lifecycle RKEK8sSystemImageLifecycle) {
	sync := NewRKEK8sSystemImageLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sSystemImageClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEK8sSystemImageLifecycle) {
	sync := NewRKEK8sSystemImageLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEK8sSystemImageHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEK8sSystemImageHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEK8sSystemImageLifecycle) {
	sync := NewRKEK8sSystemImageLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sSystemImageClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEK8sSystemImageLifecycle) {
	sync := NewRKEK8sSystemImageLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
