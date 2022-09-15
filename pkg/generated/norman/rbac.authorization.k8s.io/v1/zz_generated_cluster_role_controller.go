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
	ClusterRoleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRole",
	}
	ClusterRoleResource = metav1.APIResource{
		Name:         "clusterroles",
		SingularName: "clusterrole",
		Namespaced:   false,
		Kind:         ClusterRoleGroupVersionKind.Kind,
	}

	ClusterRoleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterroles",
	}
)

func init() {
	resource.Put(ClusterRoleGroupVersionResource)
}

// Deprecated: use v1.ClusterRole instead
type ClusterRole = v1.ClusterRole

func NewClusterRole(namespace, name string, obj v1.ClusterRole) *v1.ClusterRole {
	obj.APIVersion, obj.Kind = ClusterRoleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRoleHandlerFunc func(key string, obj *v1.ClusterRole) (runtime.Object, error)

type ClusterRoleChangeHandlerFunc func(obj *v1.ClusterRole) (runtime.Object, error)

type ClusterRoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ClusterRole, err error)
	Get(namespace, name string) (*v1.ClusterRole, error)
}

type ClusterRoleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleLister
	AddHandler(ctx context.Context, name string, handler ClusterRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterRoleHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterRoleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ClusterRole) (*v1.ClusterRole, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	Get(name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	Update(*v1.ClusterRole) (*v1.ClusterRole, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.ClusterRoleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ClusterRoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleController
	AddHandler(ctx context.Context, name string, sync ClusterRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleLifecycle)
}

type clusterRoleLister struct {
	ns         string
	controller *clusterRoleController
}

func (l *clusterRoleLister) List(namespace string, selector labels.Selector) (ret []*v1.ClusterRole, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ClusterRole))
	})
	return
}

func (l *clusterRoleLister) Get(namespace, name string) (*v1.ClusterRole, error) {
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
			Group:    ClusterRoleGroupVersionKind.Group,
			Resource: ClusterRoleGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.ClusterRole), nil
}

type clusterRoleController struct {
	ns string
	controller.GenericController
}

func (c *clusterRoleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRoleController) Lister() ClusterRoleLister {
	return &clusterRoleLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterRoleController) AddHandler(ctx context.Context, name string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ClusterRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRoleFactory struct {
}

func (c clusterRoleFactory) Object() runtime.Object {
	return &v1.ClusterRole{}
}

func (c clusterRoleFactory) List() runtime.Object {
	return &v1.ClusterRoleList{}
}

func (s *clusterRoleClient) Controller() ClusterRoleController {
	genericController := controller.NewGenericController(s.ns, ClusterRoleGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterRoleGroupVersionResource, ClusterRoleGroupVersionKind.Kind, false))

	return &clusterRoleController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterRoleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRoleController
}

func (s *clusterRoleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleClient) Create(o *v1.ClusterRole) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Get(name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Update(o *v1.ClusterRole) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) UpdateStatus(o *v1.ClusterRole) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRoleClient) List(opts metav1.ListOptions) (*v1.ClusterRoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.ClusterRoleList), err
}

func (s *clusterRoleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.ClusterRoleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.ClusterRoleList), err
}

func (s *clusterRoleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleClient) Patch(o *v1.ClusterRole, patchType types.PatchType, data []byte, subresources ...string) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleClient) AddHandler(ctx context.Context, name string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterRoleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleLifecycle) {
	sync := NewClusterRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
