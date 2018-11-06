package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type ProjectRoleTemplateBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectRoleTemplateBinding
}

type ProjectRoleTemplateBindingHandlerFunc func(key string, obj *ProjectRoleTemplateBinding) (runtime.Object, error)

type ProjectRoleTemplateBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectRoleTemplateBinding, err error)
	Get(namespace, name string) (*ProjectRoleTemplateBinding, error)
}

type ProjectRoleTemplateBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectRoleTemplateBindingLister
	AddHandler(ctx context.Context, name string, handler ProjectRoleTemplateBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectRoleTemplateBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectRoleTemplateBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectRoleTemplateBinding, error)
	Get(name string, opts metav1.GetOptions) (*ProjectRoleTemplateBinding, error)
	Update(*ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectRoleTemplateBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectRoleTemplateBindingController
	AddHandler(ctx context.Context, name string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectRoleTemplateBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectRoleTemplateBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectRoleTemplateBindingLifecycle)
}

type projectRoleTemplateBindingLister struct {
	controller *projectRoleTemplateBindingController
}

func (l *projectRoleTemplateBindingLister) List(namespace string, selector labels.Selector) (ret []*ProjectRoleTemplateBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectRoleTemplateBinding))
	})
	return
}

func (l *projectRoleTemplateBindingLister) Get(namespace, name string) (*ProjectRoleTemplateBinding, error) {
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
			Resource: "projectRoleTemplateBinding",
		}, key)
	}
	return obj.(*ProjectRoleTemplateBinding), nil
}

type projectRoleTemplateBindingController struct {
	controller.GenericController
}

func (c *projectRoleTemplateBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectRoleTemplateBindingController) Lister() ProjectRoleTemplateBindingLister {
	return &projectRoleTemplateBindingLister{
		controller: c,
	}
}

func (c *projectRoleTemplateBindingController) AddHandler(ctx context.Context, name string, handler ProjectRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectRoleTemplateBinding); ok {
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
		} else if v, ok := obj.(*ProjectRoleTemplateBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectRoleTemplateBindingFactory struct {
}

func (c projectRoleTemplateBindingFactory) Object() runtime.Object {
	return &ProjectRoleTemplateBinding{}
}

func (c projectRoleTemplateBindingFactory) List() runtime.Object {
	return &ProjectRoleTemplateBindingList{}
}

func (s *projectRoleTemplateBindingClient) Controller() ProjectRoleTemplateBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectRoleTemplateBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectRoleTemplateBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectRoleTemplateBindingController{
		GenericController: genericController,
	}

	s.client.projectRoleTemplateBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *projectRoleTemplateBindingClient) Create(o *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) Get(name string, opts metav1.GetOptions) (*ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) Update(o *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectRoleTemplateBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectRoleTemplateBindingClient) List(opts metav1.ListOptions) (*ProjectRoleTemplateBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectRoleTemplateBindingList), err
}

func (s *projectRoleTemplateBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectRoleTemplateBindingClient) Patch(o *ProjectRoleTemplateBinding, data []byte, subresources ...string) (*ProjectRoleTemplateBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ProjectRoleTemplateBinding), err
}

func (s *projectRoleTemplateBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectRoleTemplateBindingClient) AddHandler(ctx context.Context, name string, sync ProjectRoleTemplateBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectRoleTemplateBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectRoleTemplateBindingLifecycle) {
	sync := NewProjectRoleTemplateBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectRoleTemplateBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectRoleTemplateBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectRoleTemplateBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectRoleTemplateBindingLifecycle) {
	sync := NewProjectRoleTemplateBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}
