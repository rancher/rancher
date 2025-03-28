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
	OIDCClientGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "OIDCClient",
	}
	OIDCClientResource = metav1.APIResource{
		Name:         "oidcclients",
		SingularName: "oidcclient",
		Namespaced:   false,
		Kind:         OIDCClientGroupVersionKind.Kind,
	}

	OIDCClientGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "oidcclients",
	}
)

func init() {
	resource.Put(OIDCClientGroupVersionResource)
}

// Deprecated: use v3.OIDCClient instead
type OIDCClient = v3.OIDCClient

func NewOIDCClient(namespace, name string, obj v3.OIDCClient) *v3.OIDCClient {
	obj.APIVersion, obj.Kind = OIDCClientGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type OIDCClientHandlerFunc func(key string, obj *v3.OIDCClient) (runtime.Object, error)

type OIDCClientChangeHandlerFunc func(obj *v3.OIDCClient) (runtime.Object, error)

type OIDCClientLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.OIDCClient, err error)
	Get(namespace, name string) (*v3.OIDCClient, error)
}

type OIDCClientController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() OIDCClientLister
	AddHandler(ctx context.Context, name string, handler OIDCClientHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync OIDCClientHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler OIDCClientHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler OIDCClientHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type OIDCClientInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.OIDCClient) (*v3.OIDCClient, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.OIDCClient, error)
	Get(name string, opts metav1.GetOptions) (*v3.OIDCClient, error)
	Update(*v3.OIDCClient) (*v3.OIDCClient, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.OIDCClientList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.OIDCClientList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() OIDCClientController
	AddHandler(ctx context.Context, name string, sync OIDCClientHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync OIDCClientHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle OIDCClientLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle OIDCClientLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync OIDCClientHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync OIDCClientHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle OIDCClientLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle OIDCClientLifecycle)
}

type oidcClientLister struct {
	ns         string
	controller *oidcClientController
}

func (l *oidcClientLister) List(namespace string, selector labels.Selector) (ret []*v3.OIDCClient, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.OIDCClient))
	})
	return
}

func (l *oidcClientLister) Get(namespace, name string) (*v3.OIDCClient, error) {
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
			Group:    OIDCClientGroupVersionKind.Group,
			Resource: OIDCClientGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.OIDCClient), nil
}

type oidcClientController struct {
	ns string
	controller.GenericController
}

func (c *oidcClientController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *oidcClientController) Lister() OIDCClientLister {
	return &oidcClientLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *oidcClientController) AddHandler(ctx context.Context, name string, handler OIDCClientHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.OIDCClient); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *oidcClientController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler OIDCClientHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.OIDCClient); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *oidcClientController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler OIDCClientHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.OIDCClient); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *oidcClientController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler OIDCClientHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.OIDCClient); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type oidcClientFactory struct {
}

func (c oidcClientFactory) Object() runtime.Object {
	return &v3.OIDCClient{}
}

func (c oidcClientFactory) List() runtime.Object {
	return &v3.OIDCClientList{}
}

func (s *oidcClientClient) Controller() OIDCClientController {
	genericController := controller.NewGenericController(s.ns, OIDCClientGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(OIDCClientGroupVersionResource, OIDCClientGroupVersionKind.Kind, false))

	return &oidcClientController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type oidcClientClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   OIDCClientController
}

func (s *oidcClientClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *oidcClientClient) Create(o *v3.OIDCClient) (*v3.OIDCClient, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.OIDCClient), err
}

func (s *oidcClientClient) Get(name string, opts metav1.GetOptions) (*v3.OIDCClient, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.OIDCClient), err
}

func (s *oidcClientClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.OIDCClient, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.OIDCClient), err
}

func (s *oidcClientClient) Update(o *v3.OIDCClient) (*v3.OIDCClient, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.OIDCClient), err
}

func (s *oidcClientClient) UpdateStatus(o *v3.OIDCClient) (*v3.OIDCClient, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.OIDCClient), err
}

func (s *oidcClientClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *oidcClientClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *oidcClientClient) List(opts metav1.ListOptions) (*v3.OIDCClientList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.OIDCClientList), err
}

func (s *oidcClientClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.OIDCClientList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.OIDCClientList), err
}

func (s *oidcClientClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *oidcClientClient) Patch(o *v3.OIDCClient, patchType types.PatchType, data []byte, subresources ...string) (*v3.OIDCClient, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.OIDCClient), err
}

func (s *oidcClientClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *oidcClientClient) AddHandler(ctx context.Context, name string, sync OIDCClientHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *oidcClientClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync OIDCClientHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *oidcClientClient) AddLifecycle(ctx context.Context, name string, lifecycle OIDCClientLifecycle) {
	sync := NewOIDCClientLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *oidcClientClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle OIDCClientLifecycle) {
	sync := NewOIDCClientLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *oidcClientClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync OIDCClientHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *oidcClientClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync OIDCClientHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *oidcClientClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle OIDCClientLifecycle) {
	sync := NewOIDCClientLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *oidcClientClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle OIDCClientLifecycle) {
	sync := NewOIDCClientLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
