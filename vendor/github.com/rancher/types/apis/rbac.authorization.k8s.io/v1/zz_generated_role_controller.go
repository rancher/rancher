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
	RoleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Role",
	}
	RoleResource = metav1.APIResource{
		Name:         "roles",
		SingularName: "role",
		Namespaced:   true,

		Kind: RoleGroupVersionKind.Kind,
	}
)

type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Role
}

type RoleHandlerFunc func(key string, obj *v1.Role) error

type RoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Role, err error)
	Get(namespace, name string) (*v1.Role, error)
}

type RoleController interface {
	Informer() cache.SharedIndexInformer
	Lister() RoleLister
	AddHandler(handler RoleHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RoleInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.Role) (*v1.Role, error)
	Get(name string, opts metav1.GetOptions) (*v1.Role, error)
	Update(*v1.Role) (*v1.Role, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleController
	AddSyncHandler(sync RoleHandlerFunc)
	AddLifecycle(name string, lifecycle RoleLifecycle)
}

type roleLister struct {
	controller *roleController
}

func (l *roleLister) List(namespace string, selector labels.Selector) (ret []*v1.Role, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Role))
	})
	return
}

func (l *roleLister) Get(namespace, name string) (*v1.Role, error) {
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
			Group:    RoleGroupVersionKind.Group,
			Resource: "role",
		}, name)
	}
	return obj.(*v1.Role), nil
}

type roleController struct {
	controller.GenericController
}

func (c *roleController) Lister() RoleLister {
	return &roleLister{
		controller: c,
	}
}

func (c *roleController) AddHandler(handler RoleHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Role))
	})
}

type roleFactory struct {
}

func (c roleFactory) Object() runtime.Object {
	return &v1.Role{}
}

func (c roleFactory) List() runtime.Object {
	return &RoleList{}
}

func (s *roleClient) Controller() RoleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.roleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RoleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &roleController{
		GenericController: genericController,
	}

	s.client.roleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type roleClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   RoleController
}

func (s *roleClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *roleClient) Create(o *v1.Role) (*v1.Role, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Role), err
}

func (s *roleClient) Get(name string, opts metav1.GetOptions) (*v1.Role, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Role), err
}

func (s *roleClient) Update(o *v1.Role) (*v1.Role, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Role), err
}

func (s *roleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleClient) List(opts metav1.ListOptions) (*RoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RoleList), err
}

func (s *roleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *roleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleClient) AddSyncHandler(sync RoleHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *roleClient) AddLifecycle(name string, lifecycle RoleLifecycle) {
	sync := NewRoleLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
