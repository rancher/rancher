package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ClusterRoleGroupVersionKind = schema.GroupVersionKind{
		Version: "v1",
		Group:   "rbac.authorization.k8s.io",
		Kind:    "ClusterRole",
	}
	ClusterRoleResource = metav1.APIResource{
		Name:         "clusterroles",
		SingularName: "clusterrole",
		Namespaced:   false,
		Kind:         ClusterRoleGroupVersionKind.Kind,
	}
)

type ClusterRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ClusterRole
}

type ClusterRoleHandlerFunc func(key string, obj *v1.ClusterRole) error

type ClusterRoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ClusterRole, err error)
	Get(namespace, name string) (*v1.ClusterRole, error)
}

type ClusterRoleController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleLister
	AddHandler(handler ClusterRoleHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRoleInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.ClusterRole) (*v1.ClusterRole, error)
	Get(name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	Update(*v1.ClusterRole) (*v1.ClusterRole, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleController
}

type clusterRoleLister struct {
	controller *clusterRoleController
}

func (l *clusterRoleLister) List(namespace string, selector labels.Selector) (ret []*v1.ClusterRole, err error) {
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
			Resource: "clusterRole",
		}, name)
	}
	return obj.(*v1.ClusterRole), nil
}

type clusterRoleController struct {
	controller.GenericController
}

func (c *clusterRoleController) Lister() ClusterRoleLister {
	return &clusterRoleLister{
		controller: c,
	}
}

func (c *clusterRoleController) AddHandler(handler ClusterRoleHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.ClusterRole))
	})
}

type clusterRoleFactory struct {
}

func (c clusterRoleFactory) Object() runtime.Object {
	return &v1.ClusterRole{}
}

func (c clusterRoleFactory) List() runtime.Object {
	return &ClusterRoleList{}
}

func (s *clusterRoleClient) Controller() ClusterRoleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRoleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRoleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRoleController{
		GenericController: genericController,
	}

	s.client.clusterRoleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRoleClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ClusterRoleController
}

func (s *clusterRoleClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *clusterRoleClient) Update(o *v1.ClusterRole) (*v1.ClusterRole, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ClusterRole), err
}

func (s *clusterRoleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleClient) List(opts metav1.ListOptions) (*ClusterRoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRoleList), err
}

func (s *clusterRoleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *clusterRoleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}
