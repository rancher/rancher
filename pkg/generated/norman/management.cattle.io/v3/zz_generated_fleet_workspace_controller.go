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
	FleetWorkspaceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "FleetWorkspace",
	}
	FleetWorkspaceResource = metav1.APIResource{
		Name:         "fleetworkspaces",
		SingularName: "fleetworkspace",
		Namespaced:   false,
		Kind:         FleetWorkspaceGroupVersionKind.Kind,
	}

	FleetWorkspaceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "fleetworkspaces",
	}
)

func init() {
	resource.Put(FleetWorkspaceGroupVersionResource)
}

// Deprecated: use v3.FleetWorkspace instead
type FleetWorkspace = v3.FleetWorkspace

func NewFleetWorkspace(namespace, name string, obj v3.FleetWorkspace) *v3.FleetWorkspace {
	obj.APIVersion, obj.Kind = FleetWorkspaceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type FleetWorkspaceHandlerFunc func(key string, obj *v3.FleetWorkspace) (runtime.Object, error)

type FleetWorkspaceChangeHandlerFunc func(obj *v3.FleetWorkspace) (runtime.Object, error)

type FleetWorkspaceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.FleetWorkspace, err error)
	Get(namespace, name string) (*v3.FleetWorkspace, error)
}

type FleetWorkspaceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() FleetWorkspaceLister
	AddHandler(ctx context.Context, name string, handler FleetWorkspaceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync FleetWorkspaceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler FleetWorkspaceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler FleetWorkspaceHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type FleetWorkspaceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.FleetWorkspace) (*v3.FleetWorkspace, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.FleetWorkspace, error)
	Get(name string, opts metav1.GetOptions) (*v3.FleetWorkspace, error)
	Update(*v3.FleetWorkspace) (*v3.FleetWorkspace, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.FleetWorkspaceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.FleetWorkspaceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() FleetWorkspaceController
	AddHandler(ctx context.Context, name string, sync FleetWorkspaceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync FleetWorkspaceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle FleetWorkspaceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle FleetWorkspaceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync FleetWorkspaceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync FleetWorkspaceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle FleetWorkspaceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle FleetWorkspaceLifecycle)
}

type fleetWorkspaceLister struct {
	ns         string
	controller *fleetWorkspaceController
}

func (l *fleetWorkspaceLister) List(namespace string, selector labels.Selector) (ret []*v3.FleetWorkspace, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.FleetWorkspace))
	})
	return
}

func (l *fleetWorkspaceLister) Get(namespace, name string) (*v3.FleetWorkspace, error) {
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
			Group:    FleetWorkspaceGroupVersionKind.Group,
			Resource: FleetWorkspaceGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.FleetWorkspace), nil
}

type fleetWorkspaceController struct {
	ns string
	controller.GenericController
}

func (c *fleetWorkspaceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *fleetWorkspaceController) Lister() FleetWorkspaceLister {
	return &fleetWorkspaceLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *fleetWorkspaceController) AddHandler(ctx context.Context, name string, handler FleetWorkspaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.FleetWorkspace); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *fleetWorkspaceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler FleetWorkspaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.FleetWorkspace); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *fleetWorkspaceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler FleetWorkspaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.FleetWorkspace); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *fleetWorkspaceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler FleetWorkspaceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.FleetWorkspace); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type fleetWorkspaceFactory struct {
}

func (c fleetWorkspaceFactory) Object() runtime.Object {
	return &v3.FleetWorkspace{}
}

func (c fleetWorkspaceFactory) List() runtime.Object {
	return &v3.FleetWorkspaceList{}
}

func (s *fleetWorkspaceClient) Controller() FleetWorkspaceController {
	genericController := controller.NewGenericController(s.ns, FleetWorkspaceGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(FleetWorkspaceGroupVersionResource, FleetWorkspaceGroupVersionKind.Kind, false))

	return &fleetWorkspaceController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type fleetWorkspaceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   FleetWorkspaceController
}

func (s *fleetWorkspaceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *fleetWorkspaceClient) Create(o *v3.FleetWorkspace) (*v3.FleetWorkspace, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.FleetWorkspace), err
}

func (s *fleetWorkspaceClient) Get(name string, opts metav1.GetOptions) (*v3.FleetWorkspace, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.FleetWorkspace), err
}

func (s *fleetWorkspaceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.FleetWorkspace, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.FleetWorkspace), err
}

func (s *fleetWorkspaceClient) Update(o *v3.FleetWorkspace) (*v3.FleetWorkspace, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.FleetWorkspace), err
}

func (s *fleetWorkspaceClient) UpdateStatus(o *v3.FleetWorkspace) (*v3.FleetWorkspace, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.FleetWorkspace), err
}

func (s *fleetWorkspaceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *fleetWorkspaceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *fleetWorkspaceClient) List(opts metav1.ListOptions) (*v3.FleetWorkspaceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.FleetWorkspaceList), err
}

func (s *fleetWorkspaceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.FleetWorkspaceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.FleetWorkspaceList), err
}

func (s *fleetWorkspaceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *fleetWorkspaceClient) Patch(o *v3.FleetWorkspace, patchType types.PatchType, data []byte, subresources ...string) (*v3.FleetWorkspace, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.FleetWorkspace), err
}

func (s *fleetWorkspaceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *fleetWorkspaceClient) AddHandler(ctx context.Context, name string, sync FleetWorkspaceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *fleetWorkspaceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync FleetWorkspaceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *fleetWorkspaceClient) AddLifecycle(ctx context.Context, name string, lifecycle FleetWorkspaceLifecycle) {
	sync := NewFleetWorkspaceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *fleetWorkspaceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle FleetWorkspaceLifecycle) {
	sync := NewFleetWorkspaceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *fleetWorkspaceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync FleetWorkspaceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *fleetWorkspaceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync FleetWorkspaceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *fleetWorkspaceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle FleetWorkspaceLifecycle) {
	sync := NewFleetWorkspaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *fleetWorkspaceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle FleetWorkspaceLifecycle) {
	sync := NewFleetWorkspaceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
