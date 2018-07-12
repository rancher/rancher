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
	ProjectLoggingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectLogging",
	}
	ProjectLoggingResource = metav1.APIResource{
		Name:         "projectloggings",
		SingularName: "projectlogging",
		Namespaced:   true,

		Kind: ProjectLoggingGroupVersionKind.Kind,
	}
)

type ProjectLoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectLogging
}

type ProjectLoggingHandlerFunc func(key string, obj *ProjectLogging) error

type ProjectLoggingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectLogging, err error)
	Get(namespace, name string) (*ProjectLogging, error)
}

type ProjectLoggingController interface {
	Informer() cache.SharedIndexInformer
	Lister() ProjectLoggingLister
	AddHandler(name string, handler ProjectLoggingHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ProjectLoggingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectLoggingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectLogging) (*ProjectLogging, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectLogging, error)
	Get(name string, opts metav1.GetOptions) (*ProjectLogging, error)
	Update(*ProjectLogging) (*ProjectLogging, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectLoggingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectLoggingController
	AddHandler(name string, sync ProjectLoggingHandlerFunc)
	AddLifecycle(name string, lifecycle ProjectLoggingLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ProjectLoggingHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectLoggingLifecycle)
}

type projectLoggingLister struct {
	controller *projectLoggingController
}

func (l *projectLoggingLister) List(namespace string, selector labels.Selector) (ret []*ProjectLogging, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectLogging))
	})
	return
}

func (l *projectLoggingLister) Get(namespace, name string) (*ProjectLogging, error) {
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
			Group:    ProjectLoggingGroupVersionKind.Group,
			Resource: "projectLogging",
		}, key)
	}
	return obj.(*ProjectLogging), nil
}

type projectLoggingController struct {
	controller.GenericController
}

func (c *projectLoggingController) Lister() ProjectLoggingLister {
	return &projectLoggingLister{
		controller: c,
	}
}

func (c *projectLoggingController) AddHandler(name string, handler ProjectLoggingHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ProjectLogging))
	})
}

func (c *projectLoggingController) AddClusterScopedHandler(name, cluster string, handler ProjectLoggingHandlerFunc) {
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

		return handler(key, obj.(*ProjectLogging))
	})
}

type projectLoggingFactory struct {
}

func (c projectLoggingFactory) Object() runtime.Object {
	return &ProjectLogging{}
}

func (c projectLoggingFactory) List() runtime.Object {
	return &ProjectLoggingList{}
}

func (s *projectLoggingClient) Controller() ProjectLoggingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectLoggingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectLoggingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectLoggingController{
		GenericController: genericController,
	}

	s.client.projectLoggingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectLoggingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectLoggingController
}

func (s *projectLoggingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectLoggingClient) Create(o *ProjectLogging) (*ProjectLogging, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) Get(name string, opts metav1.GetOptions) (*ProjectLogging, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectLogging, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) Update(o *ProjectLogging) (*ProjectLogging, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectLoggingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectLoggingClient) List(opts metav1.ListOptions) (*ProjectLoggingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectLoggingList), err
}

func (s *projectLoggingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectLoggingClient) Patch(o *ProjectLogging, data []byte, subresources ...string) (*ProjectLogging, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectLoggingClient) AddHandler(name string, sync ProjectLoggingHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *projectLoggingClient) AddLifecycle(name string, lifecycle ProjectLoggingLifecycle) {
	sync := NewProjectLoggingLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *projectLoggingClient) AddClusterScopedHandler(name, clusterName string, sync ProjectLoggingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *projectLoggingClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectLoggingLifecycle) {
	sync := NewProjectLoggingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
