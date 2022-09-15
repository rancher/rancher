package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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
	NamespacedCertificateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedCertificate",
	}
	NamespacedCertificateResource = metav1.APIResource{
		Name:         "namespacedcertificates",
		SingularName: "namespacedcertificate",
		Namespaced:   true,

		Kind: NamespacedCertificateGroupVersionKind.Kind,
	}

	NamespacedCertificateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespacedcertificates",
	}
)

func init() {
	resource.Put(NamespacedCertificateGroupVersionResource)
}

// Deprecated: use v3.NamespacedCertificate instead
type NamespacedCertificate = v3.NamespacedCertificate

func NewNamespacedCertificate(namespace, name string, obj v3.NamespacedCertificate) *v3.NamespacedCertificate {
	obj.APIVersion, obj.Kind = NamespacedCertificateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedCertificateHandlerFunc func(key string, obj *v3.NamespacedCertificate) (runtime.Object, error)

type NamespacedCertificateChangeHandlerFunc func(obj *v3.NamespacedCertificate) (runtime.Object, error)

type NamespacedCertificateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NamespacedCertificate, err error)
	Get(namespace, name string) (*v3.NamespacedCertificate, error)
}

type NamespacedCertificateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedCertificateLister
	AddHandler(ctx context.Context, name string, handler NamespacedCertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedCertificateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedCertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedCertificateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NamespacedCertificateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NamespacedCertificate) (*v3.NamespacedCertificate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedCertificate, error)
	Get(name string, opts metav1.GetOptions) (*v3.NamespacedCertificate, error)
	Update(*v3.NamespacedCertificate) (*v3.NamespacedCertificate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NamespacedCertificateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedCertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedCertificateController
	AddHandler(ctx context.Context, name string, sync NamespacedCertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedCertificateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedCertificateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedCertificateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedCertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedCertificateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedCertificateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedCertificateLifecycle)
}

type namespacedCertificateLister struct {
	ns         string
	controller *namespacedCertificateController
}

func (l *namespacedCertificateLister) List(namespace string, selector labels.Selector) (ret []*v3.NamespacedCertificate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NamespacedCertificate))
	})
	return
}

func (l *namespacedCertificateLister) Get(namespace, name string) (*v3.NamespacedCertificate, error) {
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
			Group:    NamespacedCertificateGroupVersionKind.Group,
			Resource: NamespacedCertificateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NamespacedCertificate), nil
}

type namespacedCertificateController struct {
	ns string
	controller.GenericController
}

func (c *namespacedCertificateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedCertificateController) Lister() NamespacedCertificateLister {
	return &namespacedCertificateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *namespacedCertificateController) AddHandler(ctx context.Context, name string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedCertificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedCertificateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedCertificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedCertificateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedCertificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedCertificateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedCertificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedCertificateFactory struct {
}

func (c namespacedCertificateFactory) Object() runtime.Object {
	return &v3.NamespacedCertificate{}
}

func (c namespacedCertificateFactory) List() runtime.Object {
	return &v3.NamespacedCertificateList{}
}

func (s *namespacedCertificateClient) Controller() NamespacedCertificateController {
	genericController := controller.NewGenericController(s.ns, NamespacedCertificateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NamespacedCertificateGroupVersionResource, NamespacedCertificateGroupVersionKind.Kind, true))

	return &namespacedCertificateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type namespacedCertificateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedCertificateController
}

func (s *namespacedCertificateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedCertificateClient) Create(o *v3.NamespacedCertificate) (*v3.NamespacedCertificate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Get(name string, opts metav1.GetOptions) (*v3.NamespacedCertificate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NamespacedCertificate), err
}

func (s *namespacedCertificateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedCertificate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Update(o *v3.NamespacedCertificate) (*v3.NamespacedCertificate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NamespacedCertificate), err
}

func (s *namespacedCertificateClient) UpdateStatus(o *v3.NamespacedCertificate) (*v3.NamespacedCertificate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedCertificateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedCertificateClient) List(opts metav1.ListOptions) (*v3.NamespacedCertificateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NamespacedCertificateList), err
}

func (s *namespacedCertificateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedCertificateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NamespacedCertificateList), err
}

func (s *namespacedCertificateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedCertificateClient) Patch(o *v3.NamespacedCertificate, patchType types.PatchType, data []byte, subresources ...string) (*v3.NamespacedCertificate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NamespacedCertificate), err
}

func (s *namespacedCertificateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedCertificateClient) AddHandler(ctx context.Context, name string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedCertificateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedCertificateClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedCertificateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedCertificateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
