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
	CatalogGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Catalog",
	}
	CatalogResource = metav1.APIResource{
		Name:         "catalogs",
		SingularName: "catalog",
		Namespaced:   false,
		Kind:         CatalogGroupVersionKind.Kind,
	}
)

type CatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Catalog
}

type CatalogHandlerFunc func(key string, obj *Catalog) error

type CatalogLister interface {
	List(namespace string, selector labels.Selector) (ret []*Catalog, err error)
	Get(namespace, name string) (*Catalog, error)
}

type CatalogController interface {
	Informer() cache.SharedIndexInformer
	Lister() CatalogLister
	AddHandler(handler CatalogHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CatalogInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Catalog) (*Catalog, error)
	Get(name string, opts metav1.GetOptions) (*Catalog, error)
	Update(*Catalog) (*Catalog, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CatalogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CatalogController
	AddSyncHandler(sync CatalogHandlerFunc)
	AddLifecycle(name string, lifecycle CatalogLifecycle)
}

type catalogLister struct {
	controller *catalogController
}

func (l *catalogLister) List(namespace string, selector labels.Selector) (ret []*Catalog, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Catalog))
	})
	return
}

func (l *catalogLister) Get(namespace, name string) (*Catalog, error) {
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
			Group:    CatalogGroupVersionKind.Group,
			Resource: "catalog",
		}, name)
	}
	return obj.(*Catalog), nil
}

type catalogController struct {
	controller.GenericController
}

func (c *catalogController) Lister() CatalogLister {
	return &catalogLister{
		controller: c,
	}
}

func (c *catalogController) AddHandler(handler CatalogHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Catalog))
	})
}

type catalogFactory struct {
}

func (c catalogFactory) Object() runtime.Object {
	return &Catalog{}
}

func (c catalogFactory) List() runtime.Object {
	return &CatalogList{}
}

func (s *catalogClient) Controller() CatalogController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.catalogControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CatalogGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &catalogController{
		GenericController: genericController,
	}

	s.client.catalogControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type catalogClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   CatalogController
}

func (s *catalogClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *catalogClient) Create(o *Catalog) (*Catalog, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Catalog), err
}

func (s *catalogClient) Get(name string, opts metav1.GetOptions) (*Catalog, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Catalog), err
}

func (s *catalogClient) Update(o *Catalog) (*Catalog, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Catalog), err
}

func (s *catalogClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogClient) List(opts metav1.ListOptions) (*CatalogList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CatalogList), err
}

func (s *catalogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogClient) Patch(o *Catalog, data []byte, subresources ...string) (*Catalog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Catalog), err
}

func (s *catalogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *catalogClient) AddSyncHandler(sync CatalogHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *catalogClient) AddLifecycle(name string, lifecycle CatalogLifecycle) {
	sync := NewCatalogLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
