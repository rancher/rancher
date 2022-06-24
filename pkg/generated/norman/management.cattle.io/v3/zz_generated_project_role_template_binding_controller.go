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
	ProjectRoleTemplateBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectRoleTemplateBinding",
	}
	ProjectRoleTemplateBindingResource = metav1.APIResource{
		Name:         "projectroletemplatebindings",
		SingularName: "projectroletemplatebinding",
		Namespaced:   true,

		Kind: ProjectRoleTemplateBindingGroupVersionKind.Kind,
	}

	ProjectRoleTemplateBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectroletemplatebindings",
	}
)

func init() {
	resource.Put(ProjectRoleTemplateBindingGroupVersionResource)
}

// Deprecated: use v3.ProjectRoleTemplateBinding instead
type ProjectRoleTemplateBinding = v3.ProjectRoleTemplateBinding

func NewProjectRoleTemplateBinding(namespace, name string, obj v3.ProjectRoleTemplateBinding) *v3.ProjectRoleTemplateBinding {
	obj.APIVersion, obj.Kind = ProjectRoleTemplateBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectRoleTemplateBindingHandlerFunc func(key string, obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error)

type ProjectRoleTemplateBindingChangeHandlerFunc func(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error)

type ProjectRoleTemplateBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ProjectRoleTemplateBinding, err error)
	Get(namespace, name string) (*v3.ProjectRoleTemplateBinding, error)
}

type ProjectRoleTemplateBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectRoleTemplateBindingLister
	AddHandler(ctx context.Context, name string, handler ProjectRoleTemplateBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectRoleTemplateBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectRoleTemplateBindingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ProjectRoleTemplateBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ProjectRoleTemplateBinding, error)
	Get(name string, opts metav1.GetOptions) (*v3.ProjectRoleTemplateBinding, error)
	Update(*v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectRoleTemplateBindingController
	AddHandler(ctx context.Context, name string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectRoleTemplateBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectRoleTemplateBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectRoleTemplateBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectRoleTemplateBindingLifecycle)
}

type projectRoleTemplateBindingLister struct {
	ns         string
	controller *projectRoleTemplateBindingController
}

func (l *projectRoleTemplateBindingLister) List(namespace string, selector labels.Selector) (ret []*v3.ProjectRoleTemplateBinding, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ProjectRoleTemplateBinding))
	})
	return
}

func (l *projectRoleTemplateBindingLister) Get(namespace, name string) (*v3.ProjectRoleTemplateBinding, error) {
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
			Group:    ProjectRoleTemplateBindingGroupVersionKind.Group,
			Resource: ProjectRoleTemplateBindingGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ProjectRoleTemplateBinding), nil
}

type projectRoleTemplateBindingController struct {
	ns string
	controller.GenericController
}

func (c *projectRoleTemplateBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectRoleTemplateBindingController) Lister() ProjectRoleTemplateBindingLister {
	return &projectRoleTemplateBindingLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *projectRoleTemplateBindingController) AddHandler(ctx context.Context, name string, handler ProjectRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectRoleTemplateBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectRoleTemplateBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectRoleTemplateBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectRoleTemplateBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectRoleTemplateBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectRoleTemplateBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectRoleTemplateBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectRoleTemplateBindingFactory struct {
}

func (c projectRoleTemplateBindingFactory) Object() runtime.Object {
	return &v3.ProjectRoleTemplateBinding{}
}

func (c projectRoleTemplateBindingFactory) List() runtime.Object {
	return &v3.ProjectRoleTemplateBindingList{}
}

func (s *projectRoleTemplateBindingClient) Controller() ProjectRoleTemplateBindingController {
	genericController := controller.NewGenericController(s.ns, ProjectRoleTemplateBindingGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ProjectRoleTemplateBindingGroupVersionResource, ProjectRoleTemplateBindingGroupVersionKind.Kind, true))

	return &projectRoleTemplateBindingController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type projectRoleTemplateBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectRoleTemplateBindingController
}

func (s *projectRoleTemplateBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectRoleTemplateBindingClient) Create(o *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) Get(name string, opts metav1.GetOptions) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) Update(o *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) UpdateStatus(o *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectRoleTemplateBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectRoleTemplateBindingClient) List(opts metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ProjectRoleTemplateBindingList), err
}

func (s *projectRoleTemplateBindingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectRoleTemplateBindingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ProjectRoleTemplateBindingList), err
}

func (s *projectRoleTemplateBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectRoleTemplateBindingClient) Patch(o *v3.ProjectRoleTemplateBinding, patchType types.PatchType, data []byte, subresources ...string) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectRoleTemplateBindingClient) AddHandler(ctx context.Context, name string, sync ProjectRoleTemplateBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectRoleTemplateBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectRoleTemplateBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectRoleTemplateBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectRoleTemplateBindingLifecycle) {
	sync := NewProjectRoleTemplateBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectRoleTemplateBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectRoleTemplateBindingLifecycle) {
	sync := NewProjectRoleTemplateBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectRoleTemplateBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectRoleTemplateBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectRoleTemplateBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectRoleTemplateBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectRoleTemplateBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectRoleTemplateBindingLifecycle) {
	sync := NewProjectRoleTemplateBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectRoleTemplateBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectRoleTemplateBindingLifecycle) {
	sync := NewProjectRoleTemplateBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
