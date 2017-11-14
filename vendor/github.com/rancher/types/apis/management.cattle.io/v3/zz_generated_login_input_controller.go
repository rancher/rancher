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
	LoginInputGroupVersionKind = schema.GroupVersionKind{
		Version: "v3",
		Group:   "management.cattle.io",
		Kind:    "LoginInput",
	}
	LoginInputResource = metav1.APIResource{
		Name:         "logininputs",
		SingularName: "logininput",
		Namespaced:   false,
		Kind:         LoginInputGroupVersionKind.Kind,
	}
)

type LoginInputList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoginInput
}

type LoginInputHandlerFunc func(key string, obj *LoginInput) error

type LoginInputLister interface {
	List(namespace string, selector labels.Selector) (ret []*LoginInput, err error)
	Get(namespace, name string) (*LoginInput, error)
}

type LoginInputController interface {
	Informer() cache.SharedIndexInformer
	Lister() LoginInputLister
	AddHandler(handler LoginInputHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type LoginInputInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*LoginInput) (*LoginInput, error)
	Get(name string, opts metav1.GetOptions) (*LoginInput, error)
	Update(*LoginInput) (*LoginInput, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*LoginInputList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() LoginInputController
}

type loginInputLister struct {
	controller *loginInputController
}

func (l *loginInputLister) List(namespace string, selector labels.Selector) (ret []*LoginInput, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*LoginInput))
	})
	return
}

func (l *loginInputLister) Get(namespace, name string) (*LoginInput, error) {
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
			Group:    LoginInputGroupVersionKind.Group,
			Resource: "loginInput",
		}, name)
	}
	return obj.(*LoginInput), nil
}

type loginInputController struct {
	controller.GenericController
}

func (c *loginInputController) Lister() LoginInputLister {
	return &loginInputLister{
		controller: c,
	}
}

func (c *loginInputController) AddHandler(handler LoginInputHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*LoginInput))
	})
}

type loginInputFactory struct {
}

func (c loginInputFactory) Object() runtime.Object {
	return &LoginInput{}
}

func (c loginInputFactory) List() runtime.Object {
	return &LoginInputList{}
}

func (s *loginInputClient) Controller() LoginInputController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.loginInputControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(LoginInputGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &loginInputController{
		GenericController: genericController,
	}

	s.client.loginInputControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type loginInputClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   LoginInputController
}

func (s *loginInputClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *loginInputClient) Create(o *LoginInput) (*LoginInput, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*LoginInput), err
}

func (s *loginInputClient) Get(name string, opts metav1.GetOptions) (*LoginInput, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*LoginInput), err
}

func (s *loginInputClient) Update(o *LoginInput) (*LoginInput, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*LoginInput), err
}

func (s *loginInputClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *loginInputClient) List(opts metav1.ListOptions) (*LoginInputList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*LoginInputList), err
}

func (s *loginInputClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *loginInputClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}
