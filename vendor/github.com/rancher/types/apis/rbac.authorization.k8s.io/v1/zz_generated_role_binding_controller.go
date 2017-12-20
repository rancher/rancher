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
	RoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RoleBinding",
	}
	RoleBindingResource = metav1.APIResource{
		Name:         "rolebindings",
		SingularName: "rolebinding",
		Namespaced:   true,

		Kind: RoleBindingGroupVersionKind.Kind,
	}
)

type RoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.RoleBinding
}

type RoleBindingHandlerFunc func(key string, obj *v1.RoleBinding) error

type RoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.RoleBinding, err error)
	Get(namespace, name string) (*v1.RoleBinding, error)
}

type RoleBindingController interface {
	Informer() cache.SharedIndexInformer
	Lister() RoleBindingLister
	AddHandler(handler RoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RoleBindingInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.RoleBinding) (*v1.RoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	Update(*v1.RoleBinding) (*v1.RoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleBindingController
	AddSyncHandler(sync RoleBindingHandlerFunc)
	AddLifecycle(name string, lifecycle RoleBindingLifecycle)
}

type roleBindingLister struct {
	controller *roleBindingController
}

func (l *roleBindingLister) List(namespace string, selector labels.Selector) (ret []*v1.RoleBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.RoleBinding))
	})
	return
}

func (l *roleBindingLister) Get(namespace, name string) (*v1.RoleBinding, error) {
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
			Group:    RoleBindingGroupVersionKind.Group,
			Resource: "roleBinding",
		}, name)
	}
	return obj.(*v1.RoleBinding), nil
}

type roleBindingController struct {
	controller.GenericController
}

func (c *roleBindingController) Lister() RoleBindingLister {
	return &roleBindingLister{
		controller: c,
	}
}

func (c *roleBindingController) AddHandler(handler RoleBindingHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.RoleBinding))
	})
}

type roleBindingFactory struct {
}

func (c roleBindingFactory) Object() runtime.Object {
	return &v1.RoleBinding{}
}

func (c roleBindingFactory) List() runtime.Object {
	return &RoleBindingList{}
}

func (s *roleBindingClient) Controller() RoleBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.roleBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RoleBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &roleBindingController{
		GenericController: genericController,
	}

	s.client.roleBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type roleBindingClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   RoleBindingController
}

func (s *roleBindingClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *roleBindingClient) Create(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Get(name string, opts metav1.GetOptions) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Update(o *v1.RoleBinding) (*v1.RoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.RoleBinding), err
}

func (s *roleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleBindingClient) List(opts metav1.ListOptions) (*RoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RoleBindingList), err
}

func (s *roleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *roleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleBindingClient) AddSyncHandler(sync RoleBindingHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *roleBindingClient) AddLifecycle(name string, lifecycle RoleBindingLifecycle) {
	sync := NewRoleBindingLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
