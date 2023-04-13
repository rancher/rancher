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
	SecretGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Secret",
	}
	SecretResource = metav1.APIResource{
		Name:         "secrets",
		SingularName: "secret",
		Namespaced:   true,

		Kind: SecretGroupVersionKind.Kind,
	}

	SecretGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "secrets",
	}
)

func init() {
	resource.Put(SecretGroupVersionResource)
}

// Deprecated: use v1.Secret instead
type Secret = v1.Secret

func NewSecret(namespace, name string, obj v1.Secret) *v1.Secret {
	obj.APIVersion, obj.Kind = SecretGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SecretHandlerFunc func(key string, obj *v1.Secret) (runtime.Object, error)

type SecretChangeHandlerFunc func(obj *v1.Secret) (runtime.Object, error)

type SecretLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Secret, err error)
	Get(namespace, name string) (*v1.Secret, error)
}

type SecretController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SecretLister
	AddHandler(ctx context.Context, name string, handler SecretHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SecretHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SecretHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler SecretHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type SecretInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Secret) (*v1.Secret, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Secret, error)
	Get(name string, opts metav1.GetOptions) (*v1.Secret, error)
	Update(*v1.Secret) (*v1.Secret, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.SecretList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.SecretList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SecretController
	AddHandler(ctx context.Context, name string, sync SecretHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SecretHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SecretLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SecretLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SecretHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SecretHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SecretLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SecretLifecycle)
}

type secretLister struct {
	ns         string
	controller *secretController
}

func (l *secretLister) List(namespace string, selector labels.Selector) (ret []*v1.Secret, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Secret))
	})
	return
}

func (l *secretLister) Get(namespace, name string) (*v1.Secret, error) {
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
			Group:    SecretGroupVersionKind.Group,
			Resource: SecretGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Secret), nil
}

type secretController struct {
	ns string
	controller.GenericController
}

func (c *secretController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *secretController) Lister() SecretLister {
	return &secretLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *secretController) AddHandler(ctx context.Context, name string, handler SecretHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Secret); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *secretController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler SecretHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Secret); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *secretController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SecretHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Secret); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *secretController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler SecretHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Secret); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type secretFactory struct {
}

func (c secretFactory) Object() runtime.Object {
	return &v1.Secret{}
}

func (c secretFactory) List() runtime.Object {
	return &v1.SecretList{}
}

func (s *secretClient) Controller() SecretController {
	genericController := controller.NewGenericController(s.ns, SecretGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(SecretGroupVersionResource, SecretGroupVersionKind.Kind, true))

	return &secretController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type secretClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SecretController
}

func (s *secretClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *secretClient) Create(o *v1.Secret) (*v1.Secret, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Secret), err
}

func (s *secretClient) Get(name string, opts metav1.GetOptions) (*v1.Secret, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Secret), err
}

func (s *secretClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Secret, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Secret), err
}

func (s *secretClient) Update(o *v1.Secret) (*v1.Secret, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Secret), err
}

func (s *secretClient) UpdateStatus(o *v1.Secret) (*v1.Secret, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Secret), err
}

func (s *secretClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *secretClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *secretClient) List(opts metav1.ListOptions) (*v1.SecretList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.SecretList), err
}

func (s *secretClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.SecretList), err
}

func (s *secretClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *secretClient) Patch(o *v1.Secret, patchType types.PatchType, data []byte, subresources ...string) (*v1.Secret, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Secret), err
}

func (s *secretClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *secretClient) AddHandler(ctx context.Context, name string, sync SecretHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *secretClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SecretHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *secretClient) AddLifecycle(ctx context.Context, name string, lifecycle SecretLifecycle) {
	sync := NewSecretLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *secretClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SecretLifecycle) {
	sync := NewSecretLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *secretClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SecretHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *secretClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SecretHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *secretClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SecretLifecycle) {
	sync := NewSecretLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *secretClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SecretLifecycle) {
	sync := NewSecretLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
