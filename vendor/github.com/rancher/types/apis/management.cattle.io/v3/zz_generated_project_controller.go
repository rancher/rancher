package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project
}

type ProjectHandlerFunc func(key string, obj *Project) error

type ProjectLister interface {
	List(namespace string, selector labels.Selector) (ret []*Project, err error)
	Get(namespace, name string) (*Project, error)
}

type ProjectController interface {
	Informer() cache.SharedIndexInformer
	Lister() ProjectLister
	AddHandler(name string, handler ProjectHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ProjectHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Project) (*Project, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*Project, error)
	Get(name string, opts metav1.GetOptions) (*Project, error)
	Update(*Project) (*Project, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectController
	AddHandler(name string, sync ProjectHandlerFunc)
	AddLifecycle(name string, lifecycle ProjectLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ProjectHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectLifecycle)
}

type projectLister struct {
	controller *projectController
}

func (l *projectLister) List(namespace string, selector labels.Selector) (ret []*Project, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Project))
	})
	return
}

func (l *projectLister) Get(namespace, name string) (*Project, error) {
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
			Resource: "project",
		}, name)
	}
	return obj.(*Project), nil
}

type projectController struct {
	controller.GenericController
}

func (c *projectController) Lister() ProjectLister {
	return &projectLister{
		controller: c,
	}
}

func (c *projectController) AddHandler(name string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Project))
	})
}

func (c *projectController) AddClusterScopedHandler(name, cluster string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*Project))
	})
}

type projectFactory struct {
}

func (c projectFactory) Object() runtime.Object {
	return &Project{}
}

func (c projectFactory) List() runtime.Object {
	return &ProjectList{}
}

func (s *projectClient) Controller() ProjectController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectController{
		GenericController: genericController,
	}

	s.client.projectControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ProjectController
}

func (s *projectClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *projectClient) Create(o *Project) (*Project, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Project), err
}

func (s *projectClient) Get(name string, opts metav1.GetOptions) (*Project, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Project), err
}

func (s *projectClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*Project, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*Project), err
}

func (s *projectClient) Update(o *Project) (*Project, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Project), err
}

func (s *projectClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *projectClient) List(opts metav1.ListOptions) (*ProjectList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectList), err
}

func (s *projectClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectClient) Patch(o *Project, data []byte, subresources ...string) (*Project, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Project), err
}

func (s *projectClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectClient) AddHandler(name string, sync ProjectHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *projectClient) AddLifecycle(name string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *projectClient) AddClusterScopedHandler(name, clusterName string, sync ProjectHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
