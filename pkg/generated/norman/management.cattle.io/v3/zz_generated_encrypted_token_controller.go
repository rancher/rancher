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
	EncryptedTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "EncryptedToken",
	}
	EncryptedTokenResource = metav1.APIResource{
		Name:         "encryptedtokens",
		SingularName: "encryptedtoken",
		Namespaced:   true,

		Kind: EncryptedTokenGroupVersionKind.Kind,
	}

	EncryptedTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "encryptedtokens",
	}
)

func init() {
	resource.Put(EncryptedTokenGroupVersionResource)
}

// Deprecated: use v3.EncryptedToken instead
type EncryptedToken = v3.EncryptedToken

func NewEncryptedToken(namespace, name string, obj v3.EncryptedToken) *v3.EncryptedToken {
	obj.APIVersion, obj.Kind = EncryptedTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type EncryptedTokenHandlerFunc func(key string, obj *v3.EncryptedToken) (runtime.Object, error)

type EncryptedTokenChangeHandlerFunc func(obj *v3.EncryptedToken) (runtime.Object, error)

type EncryptedTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.EncryptedToken, err error)
	Get(namespace, name string) (*v3.EncryptedToken, error)
}

type EncryptedTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() EncryptedTokenLister
	AddHandler(ctx context.Context, name string, handler EncryptedTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EncryptedTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler EncryptedTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler EncryptedTokenHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type EncryptedTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.EncryptedToken) (*v3.EncryptedToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.EncryptedToken, error)
	Get(name string, opts metav1.GetOptions) (*v3.EncryptedToken, error)
	Update(*v3.EncryptedToken) (*v3.EncryptedToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.EncryptedTokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.EncryptedTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() EncryptedTokenController
	AddHandler(ctx context.Context, name string, sync EncryptedTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EncryptedTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle EncryptedTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EncryptedTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EncryptedTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EncryptedTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EncryptedTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EncryptedTokenLifecycle)
}

type encryptedTokenLister struct {
	ns         string
	controller *encryptedTokenController
}

func (l *encryptedTokenLister) List(namespace string, selector labels.Selector) (ret []*v3.EncryptedToken, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.EncryptedToken))
	})
	return
}

func (l *encryptedTokenLister) Get(namespace, name string) (*v3.EncryptedToken, error) {
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
			Group:    EncryptedTokenGroupVersionKind.Group,
			Resource: EncryptedTokenGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.EncryptedToken), nil
}

type encryptedTokenController struct {
	ns string
	controller.GenericController
}

func (c *encryptedTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *encryptedTokenController) Lister() EncryptedTokenLister {
	return &encryptedTokenLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *encryptedTokenController) AddHandler(ctx context.Context, name string, handler EncryptedTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EncryptedToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *encryptedTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler EncryptedTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EncryptedToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *encryptedTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler EncryptedTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EncryptedToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *encryptedTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler EncryptedTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EncryptedToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type encryptedTokenFactory struct {
}

func (c encryptedTokenFactory) Object() runtime.Object {
	return &v3.EncryptedToken{}
}

func (c encryptedTokenFactory) List() runtime.Object {
	return &v3.EncryptedTokenList{}
}

func (s *encryptedTokenClient) Controller() EncryptedTokenController {
	genericController := controller.NewGenericController(s.ns, EncryptedTokenGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(EncryptedTokenGroupVersionResource, EncryptedTokenGroupVersionKind.Kind, true))

	return &encryptedTokenController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type encryptedTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   EncryptedTokenController
}

func (s *encryptedTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *encryptedTokenClient) Create(o *v3.EncryptedToken) (*v3.EncryptedToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.EncryptedToken), err
}

func (s *encryptedTokenClient) Get(name string, opts metav1.GetOptions) (*v3.EncryptedToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.EncryptedToken), err
}

func (s *encryptedTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.EncryptedToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.EncryptedToken), err
}

func (s *encryptedTokenClient) Update(o *v3.EncryptedToken) (*v3.EncryptedToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.EncryptedToken), err
}

func (s *encryptedTokenClient) UpdateStatus(o *v3.EncryptedToken) (*v3.EncryptedToken, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.EncryptedToken), err
}

func (s *encryptedTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *encryptedTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *encryptedTokenClient) List(opts metav1.ListOptions) (*v3.EncryptedTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.EncryptedTokenList), err
}

func (s *encryptedTokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.EncryptedTokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.EncryptedTokenList), err
}

func (s *encryptedTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *encryptedTokenClient) Patch(o *v3.EncryptedToken, patchType types.PatchType, data []byte, subresources ...string) (*v3.EncryptedToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.EncryptedToken), err
}

func (s *encryptedTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *encryptedTokenClient) AddHandler(ctx context.Context, name string, sync EncryptedTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *encryptedTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EncryptedTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *encryptedTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle EncryptedTokenLifecycle) {
	sync := NewEncryptedTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *encryptedTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EncryptedTokenLifecycle) {
	sync := NewEncryptedTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *encryptedTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EncryptedTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *encryptedTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EncryptedTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *encryptedTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EncryptedTokenLifecycle) {
	sync := NewEncryptedTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *encryptedTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EncryptedTokenLifecycle) {
	sync := NewEncryptedTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
