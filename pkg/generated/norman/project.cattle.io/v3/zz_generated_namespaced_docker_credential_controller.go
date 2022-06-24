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
	NamespacedDockerCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedDockerCredential",
	}
	NamespacedDockerCredentialResource = metav1.APIResource{
		Name:         "namespaceddockercredentials",
		SingularName: "namespaceddockercredential",
		Namespaced:   true,

		Kind: NamespacedDockerCredentialGroupVersionKind.Kind,
	}

	NamespacedDockerCredentialGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "namespaceddockercredentials",
	}
)

func init() {
	resource.Put(NamespacedDockerCredentialGroupVersionResource)
}

// Deprecated: use v3.NamespacedDockerCredential instead
type NamespacedDockerCredential = v3.NamespacedDockerCredential

func NewNamespacedDockerCredential(namespace, name string, obj v3.NamespacedDockerCredential) *v3.NamespacedDockerCredential {
	obj.APIVersion, obj.Kind = NamespacedDockerCredentialGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NamespacedDockerCredentialHandlerFunc func(key string, obj *v3.NamespacedDockerCredential) (runtime.Object, error)

type NamespacedDockerCredentialChangeHandlerFunc func(obj *v3.NamespacedDockerCredential) (runtime.Object, error)

type NamespacedDockerCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NamespacedDockerCredential, err error)
	Get(namespace, name string) (*v3.NamespacedDockerCredential, error)
}

type NamespacedDockerCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NamespacedDockerCredentialLister
	AddHandler(ctx context.Context, name string, handler NamespacedDockerCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedDockerCredentialHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NamespacedDockerCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NamespacedDockerCredentialHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NamespacedDockerCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NamespacedDockerCredential) (*v3.NamespacedDockerCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedDockerCredential, error)
	Get(name string, opts metav1.GetOptions) (*v3.NamespacedDockerCredential, error)
	Update(*v3.NamespacedDockerCredential) (*v3.NamespacedDockerCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NamespacedDockerCredentialList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedDockerCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedDockerCredentialController
	AddHandler(ctx context.Context, name string, sync NamespacedDockerCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedDockerCredentialHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NamespacedDockerCredentialLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedDockerCredentialLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedDockerCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedDockerCredentialHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedDockerCredentialLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedDockerCredentialLifecycle)
}

type namespacedDockerCredentialLister struct {
	ns         string
	controller *namespacedDockerCredentialController
}

func (l *namespacedDockerCredentialLister) List(namespace string, selector labels.Selector) (ret []*v3.NamespacedDockerCredential, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NamespacedDockerCredential))
	})
	return
}

func (l *namespacedDockerCredentialLister) Get(namespace, name string) (*v3.NamespacedDockerCredential, error) {
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
			Group:    NamespacedDockerCredentialGroupVersionKind.Group,
			Resource: NamespacedDockerCredentialGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NamespacedDockerCredential), nil
}

type namespacedDockerCredentialController struct {
	ns string
	controller.GenericController
}

func (c *namespacedDockerCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *namespacedDockerCredentialController) Lister() NamespacedDockerCredentialLister {
	return &namespacedDockerCredentialLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *namespacedDockerCredentialController) AddHandler(ctx context.Context, name string, handler NamespacedDockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedDockerCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedDockerCredentialController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NamespacedDockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedDockerCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedDockerCredentialController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NamespacedDockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedDockerCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *namespacedDockerCredentialController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NamespacedDockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NamespacedDockerCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type namespacedDockerCredentialFactory struct {
}

func (c namespacedDockerCredentialFactory) Object() runtime.Object {
	return &v3.NamespacedDockerCredential{}
}

func (c namespacedDockerCredentialFactory) List() runtime.Object {
	return &v3.NamespacedDockerCredentialList{}
}

func (s *namespacedDockerCredentialClient) Controller() NamespacedDockerCredentialController {
	genericController := controller.NewGenericController(s.ns, NamespacedDockerCredentialGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NamespacedDockerCredentialGroupVersionResource, NamespacedDockerCredentialGroupVersionKind.Kind, true))

	return &namespacedDockerCredentialController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type namespacedDockerCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NamespacedDockerCredentialController
}

func (s *namespacedDockerCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *namespacedDockerCredentialClient) Create(o *v3.NamespacedDockerCredential) (*v3.NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) Get(name string, opts metav1.GetOptions) (*v3.NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NamespacedDockerCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) Update(o *v3.NamespacedDockerCredential) (*v3.NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) UpdateStatus(o *v3.NamespacedDockerCredential) (*v3.NamespacedDockerCredential, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedDockerCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *namespacedDockerCredentialClient) List(opts metav1.ListOptions) (*v3.NamespacedDockerCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NamespacedDockerCredentialList), err
}

func (s *namespacedDockerCredentialClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NamespacedDockerCredentialList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NamespacedDockerCredentialList), err
}

func (s *namespacedDockerCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedDockerCredentialClient) Patch(o *v3.NamespacedDockerCredential, patchType types.PatchType, data []byte, subresources ...string) (*v3.NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedDockerCredentialClient) AddHandler(ctx context.Context, name string, sync NamespacedDockerCredentialHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedDockerCredentialClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NamespacedDockerCredentialHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedDockerCredentialClient) AddLifecycle(ctx context.Context, name string, lifecycle NamespacedDockerCredentialLifecycle) {
	sync := NewNamespacedDockerCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *namespacedDockerCredentialClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NamespacedDockerCredentialLifecycle) {
	sync := NewNamespacedDockerCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *namespacedDockerCredentialClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NamespacedDockerCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedDockerCredentialClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NamespacedDockerCredentialHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *namespacedDockerCredentialClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NamespacedDockerCredentialLifecycle) {
	sync := NewNamespacedDockerCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *namespacedDockerCredentialClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NamespacedDockerCredentialLifecycle) {
	sync := NewNamespacedDockerCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
