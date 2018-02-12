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
	ProjectAlertGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectAlert",
	}
	ProjectAlertResource = metav1.APIResource{
		Name:         "projectalerts",
		SingularName: "projectalert",
		Namespaced:   true,

		Kind: ProjectAlertGroupVersionKind.Kind,
	}
)

type ProjectAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectAlert
}

type ProjectAlertHandlerFunc func(key string, obj *ProjectAlert) error

type ProjectAlertLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectAlert, err error)
	Get(namespace, name string) (*ProjectAlert, error)
}

type ProjectAlertController interface {
	Informer() cache.SharedIndexInformer
	Lister() ProjectAlertLister
	AddHandler(name string, handler ProjectAlertHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ProjectAlertHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectAlertInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*ProjectAlert) (*ProjectAlert, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlert, error)
	Get(name string, opts metav1.GetOptions) (*ProjectAlert, error)
	Update(*ProjectAlert) (*ProjectAlert, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectAlertList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectAlertController
	AddHandler(name string, sync ProjectAlertHandlerFunc)
	AddLifecycle(name string, lifecycle ProjectAlertLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ProjectAlertHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectAlertLifecycle)
}

type projectAlertLister struct {
	controller *projectAlertController
}

func (l *projectAlertLister) List(namespace string, selector labels.Selector) (ret []*ProjectAlert, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectAlert))
	})
	return
}

func (l *projectAlertLister) Get(namespace, name string) (*ProjectAlert, error) {
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
			Group:    ProjectAlertGroupVersionKind.Group,
			Resource: "projectAlert",
		}, name)
	}
	return obj.(*ProjectAlert), nil
}

type projectAlertController struct {
	controller.GenericController
}

func (c *projectAlertController) Lister() ProjectAlertLister {
	return &projectAlertLister{
		controller: c,
	}
}

func (c *projectAlertController) AddHandler(name string, handler ProjectAlertHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ProjectAlert))
	})
}

func (c *projectAlertController) AddClusterScopedHandler(name, cluster string, handler ProjectAlertHandlerFunc) {
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

		return handler(key, obj.(*ProjectAlert))
	})
}

type projectAlertFactory struct {
}

func (c projectAlertFactory) Object() runtime.Object {
	return &ProjectAlert{}
}

func (c projectAlertFactory) List() runtime.Object {
	return &ProjectAlertList{}
}

func (s *projectAlertClient) Controller() ProjectAlertController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectAlertControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectAlertGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectAlertController{
		GenericController: genericController,
	}

	s.client.projectAlertControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectAlertClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ProjectAlertController
}

func (s *projectAlertClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *projectAlertClient) Create(o *ProjectAlert) (*ProjectAlert, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) Get(name string, opts metav1.GetOptions) (*ProjectAlert, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlert, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) Update(o *ProjectAlert) (*ProjectAlert, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectAlertClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectAlertClient) List(opts metav1.ListOptions) (*ProjectAlertList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectAlertList), err
}

func (s *projectAlertClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectAlertClient) Patch(o *ProjectAlert, data []byte, subresources ...string) (*ProjectAlert, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectAlertClient) AddHandler(name string, sync ProjectAlertHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *projectAlertClient) AddLifecycle(name string, lifecycle ProjectAlertLifecycle) {
	sync := NewProjectAlertLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *projectAlertClient) AddClusterScopedHandler(name, clusterName string, sync ProjectAlertHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *projectAlertClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectAlertLifecycle) {
	sync := NewProjectAlertLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
