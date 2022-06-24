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
	ClusterRoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRoleBinding",
	}
	ClusterRoleBindingResource = metav1.APIResource{
		Name:         "clusterrolebindings",
		SingularName: "clusterrolebinding",
		Namespaced:   false,
		Kind:         ClusterRoleBindingGroupVersionKind.Kind,
	}

	ClusterRoleBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterrolebindings",
	}
)

func init() {
	resource.Put(ClusterRoleBindingGroupVersionResource)
}

// Deprecated: use v1.ClusterRoleBinding instead
type ClusterRoleBinding = v1.ClusterRoleBinding

func NewClusterRoleBinding(namespace, name string, obj v1.ClusterRoleBinding) *v1.ClusterRoleBinding {
	obj.APIVersion, obj.Kind = ClusterRoleBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRoleBindingHandlerFunc func(key string, obj *v1.ClusterRoleBinding) (runtime.Object, error)

type ClusterRoleBindingChangeHandlerFunc func(obj *v1.ClusterRoleBinding) (runtime.Object, error)

type ClusterRoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ClusterRoleBinding, err error)
	Get(namespace, name string) (*v1.ClusterRoleBinding, error)
}

type ClusterRoleBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleBindingLister
	AddHandler(ctx context.Context, name string, handler ClusterRoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterRoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterRoleBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	Update(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleBindingController
	AddHandler(ctx context.Context, name string, sync ClusterRoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleBindingLifecycle)
}

type clusterRoleBindingLister struct {
	ns         string
	controller *clusterRoleBindingController
}

func (l *clusterRoleBindingLister) List(namespace string, selector labels.Selector) (ret []*v1.ClusterRoleBinding, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ClusterRoleBinding))
	})
	return
}

func (l *clusterRoleBindingLister) Get(namespace, name string) (*v1.ClusterRoleBinding, error) {
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
			Group:    ClusterRoleBindingGroupVersionKind.Group,
			Resource: ClusterRoleBindingGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ClusterRoleBinding), nil
}

type clusterRoleBindingController struct {
	ns string
	controller.GenericController
}

func (c *clusterRoleBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRoleBindingController) Lister() ClusterRoleBindingLister {
	return &clusterRoleBindingLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterRoleBindingController) AddHandler(ctx context.Context, name string, handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRoleBindingFactory struct {
}

func (c clusterRoleBindingFactory) Object() runtime.Object {
	return &v1.ClusterRoleBinding{}
}

func (c clusterRoleBindingFactory) List() runtime.Object {
	return &v1.ClusterRoleBindingList{}
}

func (s *clusterRoleBindingClient) Controller() ClusterRoleBindingController {
	genericController := controller.NewGenericController(s.ns, ClusterRoleBindingGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterRoleBindingGroupVersionResource, ClusterRoleBindingGroupVersionKind.Kind, false))

	return &clusterRoleBindingController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterRoleBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRoleBindingController
}

func (s *clusterRoleBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleBindingClient) Create(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Get(name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Update(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) UpdateStatus(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRoleBindingClient) List(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ClusterRoleBindingList), err
}

func (s *clusterRoleBindingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ClusterRoleBindingList), err
}

func (s *clusterRoleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleBindingClient) Patch(o *v1.ClusterRoleBinding, patchType types.PatchType, data []byte, subresources ...string) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleBindingClient) AddHandler(ctx context.Context, name string, sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterRoleBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
