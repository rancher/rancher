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
	TokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Token",
	}
	TokenResource = metav1.APIResource{
		Name:         "tokens",
		SingularName: "token",
		Namespaced:   false,
		Kind:         TokenGroupVersionKind.Kind,
	}

	TokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "tokens",
	}
)

func init() {
	resource.Put(TokenGroupVersionResource)
}

// Deprecated: use v3.Token instead
type Token = v3.Token

func NewToken(namespace, name string, obj v3.Token) *v3.Token {
	obj.APIVersion, obj.Kind = TokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TokenHandlerFunc func(key string, obj *v3.Token) (runtime.Object, error)

type TokenChangeHandlerFunc func(obj *v3.Token) (runtime.Object, error)

type TokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Token, err error)
	Get(namespace, name string) (*v3.Token, error)
}

type TokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TokenLister
	AddHandler(ctx context.Context, name string, handler TokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler TokenHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type TokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Token) (*v3.Token, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Token, error)
	Get(name string, opts metav1.GetOptions) (*v3.Token, error)
	Update(*v3.Token) (*v3.Token, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.TokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.TokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TokenController
	AddHandler(ctx context.Context, name string, sync TokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TokenLifecycle)
}

type tokenLister struct {
	ns         string
	controller *tokenController
}

func (l *tokenLister) List(namespace string, selector labels.Selector) (ret []*v3.Token, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Token))
	})
	return
}

func (l *tokenLister) Get(namespace, name string) (*v3.Token, error) {
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
			Group:    TokenGroupVersionKind.Group,
			Resource: TokenGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Token), nil
}

type tokenController struct {
	ns string
	controller.GenericController
}

func (c *tokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *tokenController) Lister() TokenLister {
	return &tokenLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *tokenController) AddHandler(ctx context.Context, name string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Token); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *tokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Token); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *tokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Token); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *tokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Token); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type tokenFactory struct {
}

func (c tokenFactory) Object() runtime.Object {
	return &v3.Token{}
}

func (c tokenFactory) List() runtime.Object {
	return &v3.TokenList{}
}

func (s *tokenClient) Controller() TokenController {
	genericController := controller.NewGenericController(s.ns, TokenGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(TokenGroupVersionResource, TokenGroupVersionKind.Kind, false))

	return &tokenController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type tokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TokenController
}

func (s *tokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *tokenClient) Create(o *v3.Token) (*v3.Token, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Token), err
}

func (s *tokenClient) Get(name string, opts metav1.GetOptions) (*v3.Token, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Token), err
}

func (s *tokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Token, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Token), err
}

func (s *tokenClient) Update(o *v3.Token) (*v3.Token, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Token), err
}

func (s *tokenClient) UpdateStatus(o *v3.Token) (*v3.Token, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Token), err
}

func (s *tokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *tokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *tokenClient) List(opts metav1.ListOptions) (*v3.TokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.TokenList), err
}

func (s *tokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.TokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.TokenList), err
}

func (s *tokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *tokenClient) Patch(o *v3.Token, patchType types.PatchType, data []byte, subresources ...string) (*v3.Token, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Token), err
}

func (s *tokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *tokenClient) AddHandler(ctx context.Context, name string, sync TokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *tokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *tokenClient) AddLifecycle(ctx context.Context, name string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *tokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *tokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
