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
	TemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Template",
	}
	TemplateResource = metav1.APIResource{
		Name:         "templates",
		SingularName: "template",
		Namespaced:   false,
		Kind:         TemplateGroupVersionKind.Kind,
	}
)

type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template
}

type TemplateHandlerFunc func(key string, obj *Template) error

type TemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*Template, err error)
	Get(namespace, name string) (*Template, error)
}

type TemplateController interface {
	Informer() cache.SharedIndexInformer
	Lister() TemplateLister
	AddHandler(handler TemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Template) (*Template, error)
	Get(name string, opts metav1.GetOptions) (*Template, error)
	Update(*Template) (*Template, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateController
	AddSyncHandler(sync TemplateHandlerFunc)
	AddLifecycle(name string, lifecycle TemplateLifecycle)
}

type templateLister struct {
	controller *templateController
}

func (l *templateLister) List(namespace string, selector labels.Selector) (ret []*Template, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Template))
	})
	return
}

func (l *templateLister) Get(namespace, name string) (*Template, error) {
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
			Group:    TemplateGroupVersionKind.Group,
			Resource: "template",
		}, name)
	}
	return obj.(*Template), nil
}

type templateController struct {
	controller.GenericController
}

func (c *templateController) Lister() TemplateLister {
	return &templateLister{
		controller: c,
	}
}

func (c *templateController) AddHandler(handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Template))
	})
}

type templateFactory struct {
}

func (c templateFactory) Object() runtime.Object {
	return &Template{}
}

func (c templateFactory) List() runtime.Object {
	return &TemplateList{}
}

func (s *templateClient) Controller() TemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.templateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &templateController{
		GenericController: genericController,
	}

	s.client.templateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type templateClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   TemplateController
}

func (s *templateClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *templateClient) Create(o *Template) (*Template, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Template), err
}

func (s *templateClient) Get(name string, opts metav1.GetOptions) (*Template, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Template), err
}

func (s *templateClient) Update(o *Template) (*Template, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Template), err
}

func (s *templateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateClient) List(opts metav1.ListOptions) (*TemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateList), err
}

func (s *templateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *templateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateClient) AddSyncHandler(sync TemplateHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *templateClient) AddLifecycle(name string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
