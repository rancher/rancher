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
	GlobalRoleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalRole",
	}
	GlobalRoleResource = metav1.APIResource{
		Name:         "globalroles",
		SingularName: "globalrole",
		Namespaced:   false,
		Kind:         GlobalRoleGroupVersionKind.Kind,
	}

	GlobalRoleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globalroles",
	}
)

func init() {
	resource.Put(GlobalRoleGroupVersionResource)
}

// Deprecated: use v3.GlobalRole instead
type GlobalRole = v3.GlobalRole

func NewGlobalRole(namespace, name string, obj v3.GlobalRole) *v3.GlobalRole {
	obj.APIVersion, obj.Kind = GlobalRoleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalRoleHandlerFunc func(key string, obj *v3.GlobalRole) (runtime.Object, error)

type GlobalRoleChangeHandlerFunc func(obj *v3.GlobalRole) (runtime.Object, error)

type GlobalRoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.GlobalRole, err error)
	Get(namespace, name string) (*v3.GlobalRole, error)
}

type GlobalRoleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalRoleLister
	AddHandler(ctx context.Context, name string, handler GlobalRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalRoleHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type GlobalRoleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.GlobalRole) (*v3.GlobalRole, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalRole, error)
	Get(name string, opts metav1.GetOptions) (*v3.GlobalRole, error)
	Update(*v3.GlobalRole) (*v3.GlobalRole, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.GlobalRoleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalRoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalRoleController
	AddHandler(ctx context.Context, name string, sync GlobalRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalRoleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalRoleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalRoleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalRoleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalRoleLifecycle)
}

type globalRoleLister struct {
	ns         string
	controller *globalRoleController
}

func (l *globalRoleLister) List(namespace string, selector labels.Selector) (ret []*v3.GlobalRole, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.GlobalRole))
	})
	return
}

func (l *globalRoleLister) Get(namespace, name string) (*v3.GlobalRole, error) {
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
			Group:    GlobalRoleGroupVersionKind.Group,
			Resource: GlobalRoleGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.GlobalRole), nil
}

type globalRoleController struct {
	ns string
	controller.GenericController
}

func (c *globalRoleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalRoleController) Lister() GlobalRoleLister {
	return &globalRoleLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *globalRoleController) AddHandler(ctx context.Context, name string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.GlobalRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalRoleFactory struct {
}

func (c globalRoleFactory) Object() runtime.Object {
	return &v3.GlobalRole{}
}

func (c globalRoleFactory) List() runtime.Object {
	return &v3.GlobalRoleList{}
}

func (s *globalRoleClient) Controller() GlobalRoleController {
	genericController := controller.NewGenericController(s.ns, GlobalRoleGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(GlobalRoleGroupVersionResource, GlobalRoleGroupVersionKind.Kind, false))

	return &globalRoleController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type globalRoleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalRoleController
}

func (s *globalRoleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalRoleClient) Create(o *v3.GlobalRole) (*v3.GlobalRole, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.GlobalRole), err
}

func (s *globalRoleClient) Get(name string, opts metav1.GetOptions) (*v3.GlobalRole, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.GlobalRole), err
}

func (s *globalRoleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.GlobalRole, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.GlobalRole), err
}

func (s *globalRoleClient) Update(o *v3.GlobalRole) (*v3.GlobalRole, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.GlobalRole), err
}

func (s *globalRoleClient) UpdateStatus(o *v3.GlobalRole) (*v3.GlobalRole, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.GlobalRole), err
}

func (s *globalRoleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalRoleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalRoleClient) List(opts metav1.ListOptions) (*v3.GlobalRoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.GlobalRoleList), err
}

func (s *globalRoleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.GlobalRoleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.GlobalRoleList), err
}

func (s *globalRoleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalRoleClient) Patch(o *v3.GlobalRole, patchType types.PatchType, data []byte, subresources ...string) (*v3.GlobalRole, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.GlobalRole), err
}

func (s *globalRoleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalRoleClient) AddHandler(ctx context.Context, name string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalRoleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalRoleClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalRoleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalRoleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalRoleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalRoleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalRoleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
