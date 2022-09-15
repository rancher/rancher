package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/rbac/v1"
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
	RoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RoleBinding",
	}
	RoleBindingResource = metav1.APIResource{
		Name:         "rolebindings",
		SingularName: "rolebinding",
		Namespaced:   true,

		Kind: RoleBindingGroupVersionKind.Kind,
	}

	RoleBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rolebindings",
	}
)

func init() {
	resource.Put(RoleBindingGroupVersionResource)
}

// Deprecated: use v1.RoleBinding instead
type RoleBinding = v1.RoleBinding

func NewRoleBinding(namespace, name string, obj v1.RoleBinding) *v1.RoleBinding {
	obj.APIVersion, obj.Kind = RoleBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RoleBindingHandlerFunc func(key string, obj *v1.RoleBinding) (runtime.Object, error)

type RoleBindingChangeHandlerFunc func(obj *v1.RoleBinding) (runtime.Object, error)

type RoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.RoleBinding, err error)
	Get(namespace, name string) (*v1.RoleBinding, error)
}

type RoleBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RoleBindingLister
	AddHandler(ctx context.Context, name string, handler RoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type RoleBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.RoleBinding) (*v1.RoleBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	Update(*v1.RoleBinding) (*v1.RoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.RoleBindingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.RoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleBindingController
	AddHandler(ctx context.Context, name string, sync RoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RoleBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleBindingLifecycle)
}

type roleBindingLister struct {
	ns         string
	controller *roleBindingController
}

func (l *roleBindingLister) List(namespace string, selector labels.Selector) (ret []*v1.RoleBinding, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.RoleBinding))
	})
	return
}

func (l *roleBindingLister) Get(namespace, name string) (*v1.RoleBinding, error) {
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
			Group:    RoleBindingGroupVersionKind.Group,
			Resource: RoleBindingGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.RoleBinding), nil
}

type roleBindingController struct {
	ns string
	controller.GenericController
}

func (c *roleBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *roleBindingController) Lister() RoleBindingLister {
	return &roleBindingLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *roleBindingController) AddHandler(ctx context.Context, name string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.RoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type roleBindingFactory struct {
}

func (c roleBindingFactory) Object() runtime.Object {
	return &v1.RoleBinding{}
}

func (c roleBindingFactory) List() runtime.Object {
	return &v1.RoleBindingList{}
}

func (s *roleBindingClient) Controller() RoleBindingController {
	genericController := controller.NewGenericController(s.ns, RoleBindingGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(RoleBindingGroupVersionResource, RoleBindingGroupVersionKind.Kind, true))

	return &roleBindingController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type roleBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RoleBindingController
}

func (s *roleBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *roleBindingClient) Create(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Get(name string, opts metav1.GetOptions) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Update(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) UpdateStatus(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *roleBindingClient) List(opts metav1.ListOptions) (*v1.RoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.RoleBindingList), err
}

func (s *roleBindingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.RoleBindingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.RoleBindingList), err
}

func (s *roleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *roleBindingClient) Patch(o *v1.RoleBinding, patchType types.PatchType, data []byte, subresources ...string) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleBindingClient) AddHandler(ctx context.Context, name string, sync RoleBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *roleBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
