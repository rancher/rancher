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
	RkeK8sServiceOptionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RkeK8sServiceOption",
	}
	RkeK8sServiceOptionResource = metav1.APIResource{
		Name:         "rkek8sserviceoptions",
		SingularName: "rkek8sserviceoption",
		Namespaced:   true,

		Kind: RkeK8sServiceOptionGroupVersionKind.Kind,
	}

	RkeK8sServiceOptionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rkek8sserviceoptions",
	}
)

func init() {
	resource.Put(RkeK8sServiceOptionGroupVersionResource)
}

// Deprecated: use v3.RkeK8sServiceOption instead
type RkeK8sServiceOption = v3.RkeK8sServiceOption

func NewRkeK8sServiceOption(namespace, name string, obj v3.RkeK8sServiceOption) *v3.RkeK8sServiceOption {
	obj.APIVersion, obj.Kind = RkeK8sServiceOptionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RkeK8sServiceOptionHandlerFunc func(key string, obj *v3.RkeK8sServiceOption) (runtime.Object, error)

type RkeK8sServiceOptionChangeHandlerFunc func(obj *v3.RkeK8sServiceOption) (runtime.Object, error)

type RkeK8sServiceOptionLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.RkeK8sServiceOption, err error)
	Get(namespace, name string) (*v3.RkeK8sServiceOption, error)
}

type RkeK8sServiceOptionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RkeK8sServiceOptionLister
	AddHandler(ctx context.Context, name string, handler RkeK8sServiceOptionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RkeK8sServiceOptionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RkeK8sServiceOptionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RkeK8sServiceOptionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type RkeK8sServiceOptionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.RkeK8sServiceOption) (*v3.RkeK8sServiceOption, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RkeK8sServiceOption, error)
	Get(name string, opts metav1.GetOptions) (*v3.RkeK8sServiceOption, error)
	Update(*v3.RkeK8sServiceOption) (*v3.RkeK8sServiceOption, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.RkeK8sServiceOptionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RkeK8sServiceOptionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RkeK8sServiceOptionController
	AddHandler(ctx context.Context, name string, sync RkeK8sServiceOptionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RkeK8sServiceOptionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RkeK8sServiceOptionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RkeK8sServiceOptionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RkeK8sServiceOptionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RkeK8sServiceOptionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RkeK8sServiceOptionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RkeK8sServiceOptionLifecycle)
}

type rkeK8sServiceOptionLister struct {
	ns         string
	controller *rkeK8sServiceOptionController
}

func (l *rkeK8sServiceOptionLister) List(namespace string, selector labels.Selector) (ret []*v3.RkeK8sServiceOption, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.RkeK8sServiceOption))
	})
	return
}

func (l *rkeK8sServiceOptionLister) Get(namespace, name string) (*v3.RkeK8sServiceOption, error) {
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
			Group:    RkeK8sServiceOptionGroupVersionKind.Group,
			Resource: RkeK8sServiceOptionGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.RkeK8sServiceOption), nil
}

type rkeK8sServiceOptionController struct {
	ns string
	controller.GenericController
}

func (c *rkeK8sServiceOptionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rkeK8sServiceOptionController) Lister() RkeK8sServiceOptionLister {
	return &rkeK8sServiceOptionLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *rkeK8sServiceOptionController) AddHandler(ctx context.Context, name string, handler RkeK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sServiceOption); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sServiceOptionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RkeK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sServiceOption); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sServiceOptionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RkeK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sServiceOption); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sServiceOptionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RkeK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RkeK8sServiceOption); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rkeK8sServiceOptionFactory struct {
}

func (c rkeK8sServiceOptionFactory) Object() runtime.Object {
	return &v3.RkeK8sServiceOption{}
}

func (c rkeK8sServiceOptionFactory) List() runtime.Object {
	return &v3.RkeK8sServiceOptionList{}
}

func (s *rkeK8sServiceOptionClient) Controller() RkeK8sServiceOptionController {
	genericController := controller.NewGenericController(s.ns, RkeK8sServiceOptionGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(RkeK8sServiceOptionGroupVersionResource, RkeK8sServiceOptionGroupVersionKind.Kind, true))

	return &rkeK8sServiceOptionController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type rkeK8sServiceOptionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RkeK8sServiceOptionController
}

func (s *rkeK8sServiceOptionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rkeK8sServiceOptionClient) Create(o *v3.RkeK8sServiceOption) (*v3.RkeK8sServiceOption, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.RkeK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) Get(name string, opts metav1.GetOptions) (*v3.RkeK8sServiceOption, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.RkeK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RkeK8sServiceOption, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.RkeK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) Update(o *v3.RkeK8sServiceOption) (*v3.RkeK8sServiceOption, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.RkeK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) UpdateStatus(o *v3.RkeK8sServiceOption) (*v3.RkeK8sServiceOption, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.RkeK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rkeK8sServiceOptionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rkeK8sServiceOptionClient) List(opts metav1.ListOptions) (*v3.RkeK8sServiceOptionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.RkeK8sServiceOptionList), err
}

func (s *rkeK8sServiceOptionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RkeK8sServiceOptionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.RkeK8sServiceOptionList), err
}

func (s *rkeK8sServiceOptionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rkeK8sServiceOptionClient) Patch(o *v3.RkeK8sServiceOption, patchType types.PatchType, data []byte, subresources ...string) (*v3.RkeK8sServiceOption, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.RkeK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rkeK8sServiceOptionClient) AddHandler(ctx context.Context, name string, sync RkeK8sServiceOptionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RkeK8sServiceOptionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddLifecycle(ctx context.Context, name string, lifecycle RkeK8sServiceOptionLifecycle) {
	sync := NewRkeK8sServiceOptionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RkeK8sServiceOptionLifecycle) {
	sync := NewRkeK8sServiceOptionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RkeK8sServiceOptionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RkeK8sServiceOptionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RkeK8sServiceOptionLifecycle) {
	sync := NewRkeK8sServiceOptionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RkeK8sServiceOptionLifecycle) {
	sync := NewRkeK8sServiceOptionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
