package v3public

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
	AuthTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "AuthToken",
	}
	AuthTokenResource = metav1.APIResource{
		Name:         "authtokens",
		SingularName: "authtoken",
		Namespaced:   false,
		Kind:         AuthTokenGroupVersionKind.Kind,
	}

	AuthTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "authtokens",
	}
)

func init() {
	resource.Put(AuthTokenGroupVersionResource)
}

func NewAuthToken(namespace, name string, obj AuthToken) *AuthToken {
	obj.APIVersion, obj.Kind = AuthTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AuthTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthToken `json:"items"`
}

type AuthTokenHandlerFunc func(key string, obj *AuthToken) (runtime.Object, error)

type AuthTokenChangeHandlerFunc func(obj *AuthToken) (runtime.Object, error)

type AuthTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*AuthToken, err error)
	Get(namespace, name string) (*AuthToken, error)
}

type AuthTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AuthTokenLister
	AddHandler(ctx context.Context, name string, handler AuthTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AuthTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AuthTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler AuthTokenHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type AuthTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*AuthToken) (*AuthToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*AuthToken, error)
	Get(name string, opts metav1.GetOptions) (*AuthToken, error)
	Update(*AuthToken) (*AuthToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*AuthTokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*AuthTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AuthTokenController
	AddHandler(ctx context.Context, name string, sync AuthTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AuthTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AuthTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AuthTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AuthTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AuthTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AuthTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AuthTokenLifecycle)
}

type authTokenLister struct {
	controller *authTokenController
}

func (l *authTokenLister) List(namespace string, selector labels.Selector) (ret []*AuthToken, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*AuthToken))
	})
	return
}

func (l *authTokenLister) Get(namespace, name string) (*AuthToken, error) {
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
			Group:    AuthTokenGroupVersionKind.Group,
			Resource: "authToken",
		}, key)
	}
	return obj.(*AuthToken), nil
}

type authTokenController struct {
	controller.GenericController
}

func (c *authTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *authTokenController) Lister() AuthTokenLister {
	return &authTokenLister{
		controller: c,
	}
}

func (c *authTokenController) AddHandler(ctx context.Context, name string, handler AuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *authTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler AuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *authTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *authTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler AuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*AuthToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type authTokenFactory struct {
}

func (c authTokenFactory) Object() runtime.Object {
	return &AuthToken{}
}

func (c authTokenFactory) List() runtime.Object {
	return &AuthTokenList{}
}

func (s *authTokenClient) Controller() AuthTokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.authTokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(AuthTokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &authTokenController{
		GenericController: genericController,
	}

	s.client.authTokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type authTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AuthTokenController
}

func (s *authTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *authTokenClient) Create(o *AuthToken) (*AuthToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*AuthToken), err
}

func (s *authTokenClient) Get(name string, opts metav1.GetOptions) (*AuthToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*AuthToken), err
}

func (s *authTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*AuthToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*AuthToken), err
}

func (s *authTokenClient) Update(o *AuthToken) (*AuthToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*AuthToken), err
}

func (s *authTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *authTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *authTokenClient) List(opts metav1.ListOptions) (*AuthTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*AuthTokenList), err
}

func (s *authTokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*AuthTokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*AuthTokenList), err
}

func (s *authTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *authTokenClient) Patch(o *AuthToken, patchType types.PatchType, data []byte, subresources ...string) (*AuthToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*AuthToken), err
}

func (s *authTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *authTokenClient) AddHandler(ctx context.Context, name string, sync AuthTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *authTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AuthTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *authTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle AuthTokenLifecycle) {
	sync := NewAuthTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *authTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AuthTokenLifecycle) {
	sync := NewAuthTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *authTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AuthTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *authTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AuthTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *authTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AuthTokenLifecycle) {
	sync := NewAuthTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *authTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AuthTokenLifecycle) {
	sync := NewAuthTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
