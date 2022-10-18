package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/networking/v1"
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
	IngressGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Ingress",
	}
	IngressResource = metav1.APIResource{
		Name:         "ingresses",
		SingularName: "ingress",
		Namespaced:   true,

		Kind: IngressGroupVersionKind.Kind,
	}

	IngressGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "ingresses",
	}
)

func init() {
	resource.Put(IngressGroupVersionResource)
}

// Deprecated: use v1.Ingress instead
type Ingress = v1.Ingress

func NewIngress(namespace, name string, obj v1.Ingress) *v1.Ingress {
	obj.APIVersion, obj.Kind = IngressGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type IngressHandlerFunc func(key string, obj *v1.Ingress) (runtime.Object, error)

type IngressChangeHandlerFunc func(obj *v1.Ingress) (runtime.Object, error)

type IngressLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Ingress, err error)
	Get(namespace, name string) (*v1.Ingress, error)
}

type IngressController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() IngressLister
	AddHandler(ctx context.Context, name string, handler IngressHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync IngressHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler IngressHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler IngressHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type IngressInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Ingress) (*v1.Ingress, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Ingress, error)
	Get(name string, opts metav1.GetOptions) (*v1.Ingress, error)
	Update(*v1.Ingress) (*v1.Ingress, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.IngressList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.IngressList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() IngressController
	AddHandler(ctx context.Context, name string, sync IngressHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync IngressHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle IngressLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle IngressLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync IngressHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync IngressHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle IngressLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle IngressLifecycle)
}

type ingressLister struct {
	ns         string
	controller *ingressController
}

func (l *ingressLister) List(namespace string, selector labels.Selector) (ret []*v1.Ingress, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Ingress))
	})
	return
}

func (l *ingressLister) Get(namespace, name string) (*v1.Ingress, error) {
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
			Group:    IngressGroupVersionKind.Group,
			Resource: IngressGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Ingress), nil
}

type ingressController struct {
	ns string
	controller.GenericController
}

func (c *ingressController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *ingressController) Lister() IngressLister {
	return &ingressLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *ingressController) AddHandler(ctx context.Context, name string, handler IngressHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Ingress); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *ingressController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler IngressHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Ingress); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *ingressController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler IngressHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Ingress); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *ingressController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler IngressHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Ingress); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type ingressFactory struct {
}

func (c ingressFactory) Object() runtime.Object {
	return &v1.Ingress{}
}

func (c ingressFactory) List() runtime.Object {
	return &v1.IngressList{}
}

func (s *ingressClient) Controller() IngressController {
	genericController := controller.NewGenericController(s.ns, IngressGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(IngressGroupVersionResource, IngressGroupVersionKind.Kind, true))

	return &ingressController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type ingressClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   IngressController
}

func (s *ingressClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *ingressClient) Create(o *v1.Ingress) (*v1.Ingress, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Ingress), err
}

func (s *ingressClient) Get(name string, opts metav1.GetOptions) (*v1.Ingress, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Ingress), err
}

func (s *ingressClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Ingress, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Ingress), err
}

func (s *ingressClient) Update(o *v1.Ingress) (*v1.Ingress, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Ingress), err
}

func (s *ingressClient) UpdateStatus(o *v1.Ingress) (*v1.Ingress, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Ingress), err
}

func (s *ingressClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *ingressClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *ingressClient) List(opts metav1.ListOptions) (*v1.IngressList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.IngressList), err
}

func (s *ingressClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.IngressList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.IngressList), err
}

func (s *ingressClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *ingressClient) Patch(o *v1.Ingress, patchType types.PatchType, data []byte, subresources ...string) (*v1.Ingress, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Ingress), err
}

func (s *ingressClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *ingressClient) AddHandler(ctx context.Context, name string, sync IngressHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *ingressClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync IngressHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *ingressClient) AddLifecycle(ctx context.Context, name string, lifecycle IngressLifecycle) {
	sync := NewIngressLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *ingressClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle IngressLifecycle) {
	sync := NewIngressLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *ingressClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync IngressHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *ingressClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync IngressHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *ingressClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle IngressLifecycle) {
	sync := NewIngressLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *ingressClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle IngressLifecycle) {
	sync := NewIngressLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
