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
	IdentityGroupVersionKind = schema.GroupVersionKind{
		Version: "v3",
		Group:   "management.cattle.io",
		Kind:    "Identity",
	}
	IdentityResource = metav1.APIResource{
		Name:         "identities",
		SingularName: "identity",
		Namespaced:   false,
		Kind:         IdentityGroupVersionKind.Kind,
	}
)

type IdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Identity
}

type IdentityHandlerFunc func(key string, obj *Identity) error

type IdentityLister interface {
	List(namespace string, selector labels.Selector) (ret []*Identity, err error)
	Get(namespace, name string) (*Identity, error)
}

type IdentityController interface {
	Informer() cache.SharedIndexInformer
	Lister() IdentityLister
	AddHandler(handler IdentityHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type IdentityInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Identity) (*Identity, error)
	Get(name string, opts metav1.GetOptions) (*Identity, error)
	Update(*Identity) (*Identity, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*IdentityList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() IdentityController
}

type identityLister struct {
	controller *identityController
}

func (l *identityLister) List(namespace string, selector labels.Selector) (ret []*Identity, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Identity))
	})
	return
}

func (l *identityLister) Get(namespace, name string) (*Identity, error) {
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
			Group:    IdentityGroupVersionKind.Group,
			Resource: "identity",
		}, name)
	}
	return obj.(*Identity), nil
}

type identityController struct {
	controller.GenericController
}

func (c *identityController) Lister() IdentityLister {
	return &identityLister{
		controller: c,
	}
}

func (c *identityController) AddHandler(handler IdentityHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Identity))
	})
}

type identityFactory struct {
}

func (c identityFactory) Object() runtime.Object {
	return &Identity{}
}

func (c identityFactory) List() runtime.Object {
	return &IdentityList{}
}

func (s *identityClient) Controller() IdentityController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.identityControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(IdentityGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &identityController{
		GenericController: genericController,
	}

	s.client.identityControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type identityClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   IdentityController
}

func (s *identityClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *identityClient) Create(o *Identity) (*Identity, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Identity), err
}

func (s *identityClient) Get(name string, opts metav1.GetOptions) (*Identity, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Identity), err
}

func (s *identityClient) Update(o *Identity) (*Identity, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Identity), err
}

func (s *identityClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *identityClient) List(opts metav1.ListOptions) (*IdentityList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*IdentityList), err
}

func (s *identityClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *identityClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}
