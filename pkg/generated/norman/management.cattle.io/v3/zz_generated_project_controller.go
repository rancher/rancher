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
	ProjectGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Project",
	}
	ProjectResource = metav1.APIResource{
		Name:         "projects",
		SingularName: "project",
		Namespaced:   true,

		Kind: ProjectGroupVersionKind.Kind,
	}

	ProjectGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projects",
	}
)

func init() {
	resource.Put(ProjectGroupVersionResource)
}

// Deprecated: use v3.Project instead
type Project = v3.Project

func NewProject(namespace, name string, obj v3.Project) *v3.Project {
	obj.APIVersion, obj.Kind = ProjectGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectHandlerFunc func(key string, obj *v3.Project) (runtime.Object, error)

type ProjectChangeHandlerFunc func(obj *v3.Project) (runtime.Object, error)

type ProjectLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Project, err error)
	Get(namespace, name string) (*v3.Project, error)
}

type ProjectController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectLister
	AddHandler(ctx context.Context, name string, handler ProjectHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ProjectInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Project) (*v3.Project, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Project, error)
	Get(name string, opts metav1.GetOptions) (*v3.Project, error)
	Update(*v3.Project) (*v3.Project, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ProjectList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectController
	AddHandler(ctx context.Context, name string, sync ProjectHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectLifecycle)
}

type projectLister struct {
	ns         string
	controller *projectController
}

func (l *projectLister) List(namespace string, selector labels.Selector) (ret []*v3.Project, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Project))
	})
	return
}

func (l *projectLister) Get(namespace, name string) (*v3.Project, error) {
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
			Group:    ProjectGroupVersionKind.Group,
			Resource: ProjectGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Project), nil
}

type projectController struct {
	ns string
	controller.GenericController
}

func (c *projectController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectController) Lister() ProjectLister {
	return &projectLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *projectController) AddHandler(ctx context.Context, name string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Project); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Project); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Project); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Project); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectFactory struct {
}

func (c projectFactory) Object() runtime.Object {
	return &v3.Project{}
}

func (c projectFactory) List() runtime.Object {
	return &v3.ProjectList{}
}

func (s *projectClient) Controller() ProjectController {
	genericController := controller.NewGenericController(s.ns, ProjectGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ProjectGroupVersionResource, ProjectGroupVersionKind.Kind, true))

	return &projectController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type projectClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectController
}

func (s *projectClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectClient) Create(o *v3.Project) (*v3.Project, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Project), err
}

func (s *projectClient) Get(name string, opts metav1.GetOptions) (*v3.Project, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Project), err
}

func (s *projectClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Project, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Project), err
}

func (s *projectClient) Update(o *v3.Project) (*v3.Project, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Project), err
}

func (s *projectClient) UpdateStatus(o *v3.Project) (*v3.Project, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Project), err
}

func (s *projectClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectClient) List(opts metav1.ListOptions) (*v3.ProjectList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ProjectList), err
}

func (s *projectClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ProjectList), err
}

func (s *projectClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectClient) Patch(o *v3.Project, patchType types.PatchType, data []byte, subresources ...string) (*v3.Project, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Project), err
}

func (s *projectClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectClient) AddHandler(ctx context.Context, name string, sync ProjectHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
