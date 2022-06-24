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
	ProjectCatalogGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectCatalog",
	}
	ProjectCatalogResource = metav1.APIResource{
		Name:         "projectcatalogs",
		SingularName: "projectcatalog",
		Namespaced:   true,

		Kind: ProjectCatalogGroupVersionKind.Kind,
	}

	ProjectCatalogGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectcatalogs",
	}
)

func init() {
	resource.Put(ProjectCatalogGroupVersionResource)
}

// Deprecated: use v3.ProjectCatalog instead
type ProjectCatalog = v3.ProjectCatalog

func NewProjectCatalog(namespace, name string, obj v3.ProjectCatalog) *v3.ProjectCatalog {
	obj.APIVersion, obj.Kind = ProjectCatalogGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectCatalogHandlerFunc func(key string, obj *v3.ProjectCatalog) (runtime.Object, error)

type ProjectCatalogChangeHandlerFunc func(obj *v3.ProjectCatalog) (runtime.Object, error)

type ProjectCatalogLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ProjectCatalog, err error)
	Get(namespace, name string) (*v3.ProjectCatalog, error)
}

type ProjectCatalogController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectCatalogLister
	AddHandler(ctx context.Context, name string, handler ProjectCatalogHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectCatalogHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectCatalogHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectCatalogHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ProjectCatalogInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ProjectCatalog) (*v3.ProjectCatalog, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ProjectCatalog, error)
	Get(name string, opts metav1.GetOptions) (*v3.ProjectCatalog, error)
	Update(*v3.ProjectCatalog) (*v3.ProjectCatalog, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ProjectCatalogList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectCatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectCatalogController
	AddHandler(ctx context.Context, name string, sync ProjectCatalogHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectCatalogHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectCatalogLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectCatalogLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectCatalogHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectCatalogHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectCatalogLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectCatalogLifecycle)
}

type projectCatalogLister struct {
	ns         string
	controller *projectCatalogController
}

func (l *projectCatalogLister) List(namespace string, selector labels.Selector) (ret []*v3.ProjectCatalog, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ProjectCatalog))
	})
	return
}

func (l *projectCatalogLister) Get(namespace, name string) (*v3.ProjectCatalog, error) {
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
			Group:    ProjectCatalogGroupVersionKind.Group,
			Resource: ProjectCatalogGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ProjectCatalog), nil
}

type projectCatalogController struct {
	ns string
	controller.GenericController
}

func (c *projectCatalogController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectCatalogController) Lister() ProjectCatalogLister {
	return &projectCatalogLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *projectCatalogController) AddHandler(ctx context.Context, name string, handler ProjectCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectCatalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectCatalogController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectCatalog); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectCatalogController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectCatalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectCatalogController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectCatalogHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectCatalog); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectCatalogFactory struct {
}

func (c projectCatalogFactory) Object() runtime.Object {
	return &v3.ProjectCatalog{}
}

func (c projectCatalogFactory) List() runtime.Object {
	return &v3.ProjectCatalogList{}
}

func (s *projectCatalogClient) Controller() ProjectCatalogController {
	genericController := controller.NewGenericController(s.ns, ProjectCatalogGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ProjectCatalogGroupVersionResource, ProjectCatalogGroupVersionKind.Kind, true))

	return &projectCatalogController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type projectCatalogClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectCatalogController
}

func (s *projectCatalogClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectCatalogClient) Create(o *v3.ProjectCatalog) (*v3.ProjectCatalog, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ProjectCatalog), err
}

func (s *projectCatalogClient) Get(name string, opts metav1.GetOptions) (*v3.ProjectCatalog, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ProjectCatalog), err
}

func (s *projectCatalogClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ProjectCatalog, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ProjectCatalog), err
}

func (s *projectCatalogClient) Update(o *v3.ProjectCatalog) (*v3.ProjectCatalog, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ProjectCatalog), err
}

func (s *projectCatalogClient) UpdateStatus(o *v3.ProjectCatalog) (*v3.ProjectCatalog, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ProjectCatalog), err
}

func (s *projectCatalogClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectCatalogClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectCatalogClient) List(opts metav1.ListOptions) (*v3.ProjectCatalogList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ProjectCatalogList), err
}

func (s *projectCatalogClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectCatalogList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ProjectCatalogList), err
}

func (s *projectCatalogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectCatalogClient) Patch(o *v3.ProjectCatalog, patchType types.PatchType, data []byte, subresources ...string) (*v3.ProjectCatalog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ProjectCatalog), err
}

func (s *projectCatalogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectCatalogClient) AddHandler(ctx context.Context, name string, sync ProjectCatalogHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectCatalogClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectCatalogHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectCatalogClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectCatalogLifecycle) {
	sync := NewProjectCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectCatalogClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectCatalogLifecycle) {
	sync := NewProjectCatalogLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectCatalogClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectCatalogHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectCatalogClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectCatalogHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectCatalogClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectCatalogLifecycle) {
	sync := NewProjectCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectCatalogClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectCatalogLifecycle) {
	sync := NewProjectCatalogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
