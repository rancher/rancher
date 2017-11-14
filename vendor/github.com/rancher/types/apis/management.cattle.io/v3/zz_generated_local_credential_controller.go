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
	LocalCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: "v3",
		Group:   "management.cattle.io",
		Kind:    "LocalCredential",
	}
	LocalCredentialResource = metav1.APIResource{
		Name:         "localcredentials",
		SingularName: "localcredential",
		Namespaced:   false,
		Kind:         LocalCredentialGroupVersionKind.Kind,
	}
)

type LocalCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LocalCredential
}

type LocalCredentialHandlerFunc func(key string, obj *LocalCredential) error

type LocalCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*LocalCredential, err error)
	Get(namespace, name string) (*LocalCredential, error)
}

type LocalCredentialController interface {
	Informer() cache.SharedIndexInformer
	Lister() LocalCredentialLister
	AddHandler(handler LocalCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type LocalCredentialInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*LocalCredential) (*LocalCredential, error)
	Get(name string, opts metav1.GetOptions) (*LocalCredential, error)
	Update(*LocalCredential) (*LocalCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*LocalCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() LocalCredentialController
}

type localCredentialLister struct {
	controller *localCredentialController
}

func (l *localCredentialLister) List(namespace string, selector labels.Selector) (ret []*LocalCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*LocalCredential))
	})
	return
}

func (l *localCredentialLister) Get(namespace, name string) (*LocalCredential, error) {
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
			Group:    LocalCredentialGroupVersionKind.Group,
			Resource: "localCredential",
		}, name)
	}
	return obj.(*LocalCredential), nil
}

type localCredentialController struct {
	controller.GenericController
}

func (c *localCredentialController) Lister() LocalCredentialLister {
	return &localCredentialLister{
		controller: c,
	}
}

func (c *localCredentialController) AddHandler(handler LocalCredentialHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*LocalCredential))
	})
}

type localCredentialFactory struct {
}

func (c localCredentialFactory) Object() runtime.Object {
	return &LocalCredential{}
}

func (c localCredentialFactory) List() runtime.Object {
	return &LocalCredentialList{}
}

func (s *localCredentialClient) Controller() LocalCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.localCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(LocalCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &localCredentialController{
		GenericController: genericController,
	}

	s.client.localCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type localCredentialClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   LocalCredentialController
}

func (s *localCredentialClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *localCredentialClient) Create(o *LocalCredential) (*LocalCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*LocalCredential), err
}

func (s *localCredentialClient) Get(name string, opts metav1.GetOptions) (*LocalCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*LocalCredential), err
}

func (s *localCredentialClient) Update(o *LocalCredential) (*LocalCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*LocalCredential), err
}

func (s *localCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *localCredentialClient) List(opts metav1.ListOptions) (*LocalCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*LocalCredentialList), err
}

func (s *localCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *localCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}
