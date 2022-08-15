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
	GlobalRoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalRoleBinding",
	}
	GlobalRoleBindingResource = metav1.APIResource{
		Name:         "globalrolebindings",
		SingularName: "globalrolebinding",
		Namespaced:   false,
		Kind:         GlobalRoleBindingGroupVersionKind.Kind,
	}

	GlobalRoleBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globalrolebindings",
	}
)

func init() {
	resource.Put(GlobalRoleBindingGroupVersionResource)
}

// Deprecated: use v3.GlobalRoleBinding instead
type GlobalRoleBinding = v3.GlobalRoleBinding

func NewGlobalRoleBinding(namespace, name string, obj v3.GlobalRoleBinding) *v3.GlobalRoleBinding {
	obj.APIVersion, obj.Kind = GlobalRoleBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalRoleBindingHandlerFunc func(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error)

type GlobalRoleBindingChangeHandlerFunc func(obj *v3.GlobalRoleBinding) (runtime.Object, error)

type GlobalRoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.GlobalRoleBinding, err error)
	Get(namespace, name string) (*v3.GlobalRoleBinding, error)
}

type GlobalRoleBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalRoleBindingLister
	AddHandler(ctx context.Context, name string, handler GlobalRoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalRoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalRoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type GlobalRoleBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalRoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v3.GlobalRoleBinding, error)
	Update(*v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.GlobalRoleBindingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalRoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalRoleBindingController
	AddHandler(ctx context.Context, name string, sync GlobalRoleBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalRoleBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalRoleBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalRoleBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalRoleBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalRoleBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalRoleBindingLifecycle)
}

type globalRoleBindingLister struct {
	ns         string
	controller *globalRoleBindingController
}

func (l *globalRoleBindingLister) List(namespace string, selector labels.Selector) (ret []*v3.GlobalRoleBinding, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.GlobalRoleBinding))
	})
	return
}

func (l *globalRoleBindingLister) Get(namespace, name string) (*v3.GlobalRoleBinding, error) {
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
			Group:    GlobalRoleBindingGroupVersionKind.Group,
			Resource: GlobalRoleBindingGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.GlobalRoleBinding), nil
}

type globalRoleBindingController struct {
	ns string
	controller.GenericController
}

func (c *globalRoleBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalRoleBindingController) Lister() GlobalRoleBindingLister {
	return &globalRoleBindingLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *globalRoleBindingController) AddHandler(ctx context.Context, name string, handler GlobalRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRoleBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRoleBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalRoleBindingFactory struct {
}

func (c globalRoleBindingFactory) Object() runtime.Object {
	return &v3.GlobalRoleBinding{}
}

func (c globalRoleBindingFactory) List() runtime.Object {
	return &v3.GlobalRoleBindingList{}
}

func (s *globalRoleBindingClient) Controller() GlobalRoleBindingController {
	genericController := controller.NewGenericController(s.ns, GlobalRoleBindingGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(GlobalRoleBindingGroupVersionResource, GlobalRoleBindingGroupVersionKind.Kind, false))

	return &globalRoleBindingController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type globalRoleBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalRoleBindingController
}

func (s *globalRoleBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalRoleBindingClient) Create(o *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) Get(name string, opts metav1.GetOptions) (*v3.GlobalRoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalRoleBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) Update(o *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) UpdateStatus(o *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalRoleBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalRoleBindingClient) List(opts metav1.ListOptions) (*v3.GlobalRoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.GlobalRoleBindingList), err
}

func (s *globalRoleBindingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalRoleBindingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.GlobalRoleBindingList), err
}

func (s *globalRoleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalRoleBindingClient) Patch(o *v3.GlobalRoleBinding, patchType types.PatchType, data []byte, subresources ...string) (*v3.GlobalRoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalRoleBindingClient) AddHandler(ctx context.Context, name string, sync GlobalRoleBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalRoleBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalRoleBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalRoleBindingLifecycle) {
	sync := NewGlobalRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalRoleBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalRoleBindingLifecycle) {
	sync := NewGlobalRoleBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalRoleBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalRoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalRoleBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalRoleBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalRoleBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalRoleBindingLifecycle) {
	sync := NewGlobalRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalRoleBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalRoleBindingLifecycle) {
	sync := NewGlobalRoleBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
