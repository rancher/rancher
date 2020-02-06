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
	ClusterRegistrationTokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRegistrationToken",
	}
	ClusterRegistrationTokenResource = metav1.APIResource{
		Name:         "clusterregistrationtokens",
		SingularName: "clusterregistrationtoken",
		Namespaced:   true,

		Kind: ClusterRegistrationTokenGroupVersionKind.Kind,
	}

	ClusterRegistrationTokenGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterregistrationtokens",
	}
)

func init() {
	resource.Put(ClusterRegistrationTokenGroupVersionResource)
}

func NewClusterRegistrationToken(namespace, name string, obj ClusterRegistrationToken) *ClusterRegistrationToken {
	obj.APIVersion, obj.Kind = ClusterRegistrationTokenGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRegistrationTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistrationToken `json:"items"`
}

type ClusterRegistrationTokenHandlerFunc func(key string, obj *ClusterRegistrationToken) (runtime.Object, error)

type ClusterRegistrationTokenChangeHandlerFunc func(obj *ClusterRegistrationToken) (runtime.Object, error)

type ClusterRegistrationTokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterRegistrationToken, err error)
	Get(namespace, name string) (*ClusterRegistrationToken, error)
}

type ClusterRegistrationTokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRegistrationTokenLister
	AddHandler(ctx context.Context, name string, handler ClusterRegistrationTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRegistrationTokenHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRegistrationTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterRegistrationTokenHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRegistrationTokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterRegistrationToken) (*ClusterRegistrationToken, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRegistrationToken, error)
	Get(name string, opts metav1.GetOptions) (*ClusterRegistrationToken, error)
	Update(*ClusterRegistrationToken) (*ClusterRegistrationToken, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRegistrationTokenList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterRegistrationTokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRegistrationTokenController
	AddHandler(ctx context.Context, name string, sync ClusterRegistrationTokenHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRegistrationTokenHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRegistrationTokenLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRegistrationTokenLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRegistrationTokenHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRegistrationTokenHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRegistrationTokenLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRegistrationTokenLifecycle)
}

type clusterRegistrationTokenLister struct {
	controller *clusterRegistrationTokenController
}

func (l *clusterRegistrationTokenLister) List(namespace string, selector labels.Selector) (ret []*ClusterRegistrationToken, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterRegistrationToken))
	})
	return
}

func (l *clusterRegistrationTokenLister) Get(namespace, name string) (*ClusterRegistrationToken, error) {
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
			Group:    ClusterRegistrationTokenGroupVersionKind.Group,
			Resource: "clusterRegistrationToken",
		}, key)
	}
	return obj.(*ClusterRegistrationToken), nil
}

type clusterRegistrationTokenController struct {
	controller.GenericController
}

func (c *clusterRegistrationTokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRegistrationTokenController) Lister() ClusterRegistrationTokenLister {
	return &clusterRegistrationTokenLister{
		controller: c,
	}
}

func (c *clusterRegistrationTokenController) AddHandler(ctx context.Context, name string, handler ClusterRegistrationTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRegistrationToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRegistrationTokenController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterRegistrationTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRegistrationToken); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRegistrationTokenController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRegistrationTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRegistrationToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRegistrationTokenController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterRegistrationTokenHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRegistrationToken); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRegistrationTokenFactory struct {
}

func (c clusterRegistrationTokenFactory) Object() runtime.Object {
	return &ClusterRegistrationToken{}
}

func (c clusterRegistrationTokenFactory) List() runtime.Object {
	return &ClusterRegistrationTokenList{}
}

func (s *clusterRegistrationTokenClient) Controller() ClusterRegistrationTokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRegistrationTokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRegistrationTokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRegistrationTokenController{
		GenericController: genericController,
	}

	s.client.clusterRegistrationTokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRegistrationTokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRegistrationTokenController
}

func (s *clusterRegistrationTokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRegistrationTokenClient) Create(o *ClusterRegistrationToken) (*ClusterRegistrationToken, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterRegistrationToken), err
}

func (s *clusterRegistrationTokenClient) Get(name string, opts metav1.GetOptions) (*ClusterRegistrationToken, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterRegistrationToken), err
}

func (s *clusterRegistrationTokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRegistrationToken, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterRegistrationToken), err
}

func (s *clusterRegistrationTokenClient) Update(o *ClusterRegistrationToken) (*ClusterRegistrationToken, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterRegistrationToken), err
}

func (s *clusterRegistrationTokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRegistrationTokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRegistrationTokenClient) List(opts metav1.ListOptions) (*ClusterRegistrationTokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRegistrationTokenList), err
}

func (s *clusterRegistrationTokenClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterRegistrationTokenList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterRegistrationTokenList), err
}

func (s *clusterRegistrationTokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRegistrationTokenClient) Patch(o *ClusterRegistrationToken, patchType types.PatchType, data []byte, subresources ...string) (*ClusterRegistrationToken, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterRegistrationToken), err
}

func (s *clusterRegistrationTokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRegistrationTokenClient) AddHandler(ctx context.Context, name string, sync ClusterRegistrationTokenHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRegistrationTokenClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRegistrationTokenHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRegistrationTokenClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRegistrationTokenLifecycle) {
	sync := NewClusterRegistrationTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRegistrationTokenClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRegistrationTokenLifecycle) {
	sync := NewClusterRegistrationTokenLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRegistrationTokenClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRegistrationTokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRegistrationTokenClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRegistrationTokenHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterRegistrationTokenClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRegistrationTokenLifecycle) {
	sync := NewClusterRegistrationTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRegistrationTokenClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRegistrationTokenLifecycle) {
	sync := NewClusterRegistrationTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
