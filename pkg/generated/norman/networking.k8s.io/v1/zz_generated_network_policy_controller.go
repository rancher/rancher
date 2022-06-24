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
	NetworkPolicyGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NetworkPolicy",
	}
	NetworkPolicyResource = metav1.APIResource{
		Name:         "networkpolicies",
		SingularName: "networkpolicy",
		Namespaced:   true,

		Kind: NetworkPolicyGroupVersionKind.Kind,
	}

	NetworkPolicyGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "networkpolicies",
	}
)

func init() {
	resource.Put(NetworkPolicyGroupVersionResource)
}

// Deprecated: use v1.NetworkPolicy instead
type NetworkPolicy = v1.NetworkPolicy

func NewNetworkPolicy(namespace, name string, obj v1.NetworkPolicy) *v1.NetworkPolicy {
	obj.APIVersion, obj.Kind = NetworkPolicyGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NetworkPolicyHandlerFunc func(key string, obj *v1.NetworkPolicy) (runtime.Object, error)

type NetworkPolicyChangeHandlerFunc func(obj *v1.NetworkPolicy) (runtime.Object, error)

type NetworkPolicyLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.NetworkPolicy, err error)
	Get(namespace, name string) (*v1.NetworkPolicy, error)
}

type NetworkPolicyController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NetworkPolicyLister
	AddHandler(ctx context.Context, name string, handler NetworkPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NetworkPolicyHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NetworkPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NetworkPolicyHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NetworkPolicyInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.NetworkPolicy) (*v1.NetworkPolicy, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.NetworkPolicy, error)
	Get(name string, opts metav1.GetOptions) (*v1.NetworkPolicy, error)
	Update(*v1.NetworkPolicy) (*v1.NetworkPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.NetworkPolicyList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.NetworkPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NetworkPolicyController
	AddHandler(ctx context.Context, name string, sync NetworkPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NetworkPolicyHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NetworkPolicyLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NetworkPolicyLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NetworkPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NetworkPolicyHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NetworkPolicyLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NetworkPolicyLifecycle)
}

type networkPolicyLister struct {
	ns         string
	controller *networkPolicyController
}

func (l *networkPolicyLister) List(namespace string, selector labels.Selector) (ret []*v1.NetworkPolicy, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.NetworkPolicy))
	})
	return
}

func (l *networkPolicyLister) Get(namespace, name string) (*v1.NetworkPolicy, error) {
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
			Group:    NetworkPolicyGroupVersionKind.Group,
			Resource: NetworkPolicyGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.NetworkPolicy), nil
}

type networkPolicyController struct {
	ns string
	controller.GenericController
}

func (c *networkPolicyController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *networkPolicyController) Lister() NetworkPolicyLister {
	return &networkPolicyLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *networkPolicyController) AddHandler(ctx context.Context, name string, handler NetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.NetworkPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *networkPolicyController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.NetworkPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *networkPolicyController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.NetworkPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *networkPolicyController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.NetworkPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type networkPolicyFactory struct {
}

func (c networkPolicyFactory) Object() runtime.Object {
	return &v1.NetworkPolicy{}
}

func (c networkPolicyFactory) List() runtime.Object {
	return &v1.NetworkPolicyList{}
}

func (s *networkPolicyClient) Controller() NetworkPolicyController {
	genericController := controller.NewGenericController(s.ns, NetworkPolicyGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NetworkPolicyGroupVersionResource, NetworkPolicyGroupVersionKind.Kind, true))

	return &networkPolicyController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type networkPolicyClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NetworkPolicyController
}

func (s *networkPolicyClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *networkPolicyClient) Create(o *v1.NetworkPolicy) (*v1.NetworkPolicy, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.NetworkPolicy), err
}

func (s *networkPolicyClient) Get(name string, opts metav1.GetOptions) (*v1.NetworkPolicy, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.NetworkPolicy), err
}

func (s *networkPolicyClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.NetworkPolicy, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.NetworkPolicy), err
}

func (s *networkPolicyClient) Update(o *v1.NetworkPolicy) (*v1.NetworkPolicy, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.NetworkPolicy), err
}

func (s *networkPolicyClient) UpdateStatus(o *v1.NetworkPolicy) (*v1.NetworkPolicy, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.NetworkPolicy), err
}

func (s *networkPolicyClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *networkPolicyClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *networkPolicyClient) List(opts metav1.ListOptions) (*v1.NetworkPolicyList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.NetworkPolicyList), err
}

func (s *networkPolicyClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.NetworkPolicyList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.NetworkPolicyList), err
}

func (s *networkPolicyClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *networkPolicyClient) Patch(o *v1.NetworkPolicy, patchType types.PatchType, data []byte, subresources ...string) (*v1.NetworkPolicy, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.NetworkPolicy), err
}

func (s *networkPolicyClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *networkPolicyClient) AddHandler(ctx context.Context, name string, sync NetworkPolicyHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *networkPolicyClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NetworkPolicyHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *networkPolicyClient) AddLifecycle(ctx context.Context, name string, lifecycle NetworkPolicyLifecycle) {
	sync := NewNetworkPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *networkPolicyClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NetworkPolicyLifecycle) {
	sync := NewNetworkPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *networkPolicyClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NetworkPolicyHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *networkPolicyClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NetworkPolicyHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *networkPolicyClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NetworkPolicyLifecycle) {
	sync := NewNetworkPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *networkPolicyClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NetworkPolicyLifecycle) {
	sync := NewNetworkPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
