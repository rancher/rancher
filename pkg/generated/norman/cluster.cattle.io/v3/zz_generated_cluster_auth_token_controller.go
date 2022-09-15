package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
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
	ClusterAuthTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterAuthToken",
	}
	ClusterAuthTokenResource = metav1.APIResource{
		Name:         "clusterauthtokens",
		SingularName: "clusterauthtoken",
		Namespaced:   true,

		Kind: ClusterAuthTokenGroupVersionKind.Kind,
	}

	ClusterAuthTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterauthtokens",
	}
)

func init() {
	resource.Put(ClusterAuthTokenGroupVersionResource)
}

// Deprecated: use v3.ClusterAuthToken instead
type ClusterAuthToken = v3.ClusterAuthToken

func NewClusterAuthToken(namespace, name string, obj v3.ClusterAuthToken) *v3.ClusterAuthToken {
	obj.APIVersion, obj.Kind = ClusterAuthTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterAuthTokenHandlerFunc func(key string, obj *v3.ClusterAuthToken) (runtime.Object, error)

type ClusterAuthTokenChangeHandlerFunc func(obj *v3.ClusterAuthToken) (runtime.Object, error)

type ClusterAuthTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ClusterAuthToken, err error)
	Get(namespace, name string) (*v3.ClusterAuthToken, error)
}

type ClusterAuthTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterAuthTokenLister
	AddHandler(ctx context.Context, name string, handler ClusterAuthTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAuthTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterAuthTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterAuthTokenHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterAuthTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ClusterAuthToken) (*v3.ClusterAuthToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterAuthToken, error)
	Get(name string, opts metav1.GetOptions) (*v3.ClusterAuthToken, error)
	Update(*v3.ClusterAuthToken) (*v3.ClusterAuthToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ClusterAuthTokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterAuthTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterAuthTokenController
	AddHandler(ctx context.Context, name string, sync ClusterAuthTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAuthTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterAuthTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAuthTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAuthTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAuthTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAuthTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAuthTokenLifecycle)
}

type clusterAuthTokenLister struct {
	ns         string
	controller *clusterAuthTokenController
}

func (l *clusterAuthTokenLister) List(namespace string, selector labels.Selector) (ret []*v3.ClusterAuthToken, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ClusterAuthToken))
	})
	return
}

func (l *clusterAuthTokenLister) Get(namespace, name string) (*v3.ClusterAuthToken, error) {
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
			Group:    ClusterAuthTokenGroupVersionKind.Group,
			Resource: ClusterAuthTokenGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ClusterAuthToken), nil
}

type clusterAuthTokenController struct {
	ns string
	controller.GenericController
}

func (c *clusterAuthTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterAuthTokenController) Lister() ClusterAuthTokenLister {
	return &clusterAuthTokenLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterAuthTokenController) AddHandler(ctx context.Context, name string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAuthToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAuthTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAuthToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAuthTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAuthToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAuthTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterAuthTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAuthToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterAuthTokenFactory struct {
}

func (c clusterAuthTokenFactory) Object() runtime.Object {
	return &v3.ClusterAuthToken{}
}

func (c clusterAuthTokenFactory) List() runtime.Object {
	return &v3.ClusterAuthTokenList{}
}

func (s *clusterAuthTokenClient) Controller() ClusterAuthTokenController {
	genericController := controller.NewGenericController(s.ns, ClusterAuthTokenGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterAuthTokenGroupVersionResource, ClusterAuthTokenGroupVersionKind.Kind, true))

	return &clusterAuthTokenController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterAuthTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterAuthTokenController
}

func (s *clusterAuthTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterAuthTokenClient) Create(o *v3.ClusterAuthToken) (*v3.ClusterAuthToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) Get(name string, opts metav1.GetOptions) (*v3.ClusterAuthToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterAuthToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) Update(o *v3.ClusterAuthToken) (*v3.ClusterAuthToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) UpdateStatus(o *v3.ClusterAuthToken) (*v3.ClusterAuthToken, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterAuthTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterAuthTokenClient) List(opts metav1.ListOptions) (*v3.ClusterAuthTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ClusterAuthTokenList), err
}

func (s *clusterAuthTokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterAuthTokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ClusterAuthTokenList), err
}

func (s *clusterAuthTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAuthTokenClient) Patch(o *v3.ClusterAuthToken, patchType types.PatchType, data []byte, subresources ...string) (*v3.ClusterAuthToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ClusterAuthToken), err
}

func (s *clusterAuthTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterAuthTokenClient) AddHandler(ctx context.Context, name string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAuthTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAuthTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAuthTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAuthTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAuthTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAuthTokenLifecycle) {
	sync := NewClusterAuthTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
