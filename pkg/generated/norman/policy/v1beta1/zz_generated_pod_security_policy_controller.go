package v1beta1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/policy/v1beta1"
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
	PodSecurityPolicyGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicy",
	}
	PodSecurityPolicyResource = metav1.APIResource{
		Name:         "podsecuritypolicies",
		SingularName: "podsecuritypolicy",
		Namespaced:   false,
		Kind:         PodSecurityPolicyGroupVersionKind.Kind,
	}

	PodSecurityPolicyGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "podsecuritypolicies",
	}
)

func init() {
	resource.Put(PodSecurityPolicyGroupVersionResource)
}

// Deprecated: use v1beta1.PodSecurityPolicy instead
type PodSecurityPolicy = v1beta1.PodSecurityPolicy

func NewPodSecurityPolicy(namespace, name string, obj v1beta1.PodSecurityPolicy) *v1beta1.PodSecurityPolicy {
	obj.APIVersion, obj.Kind = PodSecurityPolicyGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodSecurityPolicyHandlerFunc func(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)

type PodSecurityPolicyChangeHandlerFunc func(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)

type PodSecurityPolicyLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error)
	Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error)
}

type PodSecurityPolicyController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyLister
	AddHandler(ctx context.Context, name string, handler PodSecurityPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodSecurityPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodSecurityPolicyHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PodSecurityPolicyInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Get(name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Update(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1beta1.PodSecurityPolicyList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1beta1.PodSecurityPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyController
	AddHandler(ctx context.Context, name string, sync PodSecurityPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyLifecycle)
}

type podSecurityPolicyLister struct {
	ns         string
	controller *podSecurityPolicyController
}

func (l *podSecurityPolicyLister) List(namespace string, selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta1.PodSecurityPolicy))
	})
	return
}

func (l *podSecurityPolicyLister) Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error) {
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
			Group:    PodSecurityPolicyGroupVersionKind.Group,
			Resource: PodSecurityPolicyGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1beta1.PodSecurityPolicy), nil
}

type podSecurityPolicyController struct {
	ns string
	controller.GenericController
}

func (c *podSecurityPolicyController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podSecurityPolicyController) Lister() PodSecurityPolicyLister {
	return &podSecurityPolicyLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *podSecurityPolicyController) AddHandler(ctx context.Context, name string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podSecurityPolicyFactory struct {
}

func (c podSecurityPolicyFactory) Object() runtime.Object {
	return &v1beta1.PodSecurityPolicy{}
}

func (c podSecurityPolicyFactory) List() runtime.Object {
	return &v1beta1.PodSecurityPolicyList{}
}

func (s *podSecurityPolicyClient) Controller() PodSecurityPolicyController {
	genericController := controller.NewGenericController(s.ns, PodSecurityPolicyGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PodSecurityPolicyGroupVersionResource, PodSecurityPolicyGroupVersionKind.Kind, false))

	return &podSecurityPolicyController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type podSecurityPolicyClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodSecurityPolicyController
}

func (s *podSecurityPolicyClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyClient) Create(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Get(name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Update(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) UpdateStatus(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityPolicyClient) List(opts metav1.ListOptions) (*v1beta1.PodSecurityPolicyList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1beta1.PodSecurityPolicyList), err
}

func (s *podSecurityPolicyClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1beta1.PodSecurityPolicyList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1beta1.PodSecurityPolicyList), err
}

func (s *podSecurityPolicyClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyClient) Patch(o *v1beta1.PodSecurityPolicy, patchType types.PatchType, data []byte, subresources ...string) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyClient) AddHandler(ctx context.Context, name string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyClient) AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
