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
	TemplateVersionGroupVersionKind = schema.GroupVersionKind{
		Version: "v3",
		Group:   "management.cattle.io",
		Kind:    "TemplateVersion",
	}
	TemplateVersionResource = metav1.APIResource{
		Name:         "templateversions",
		SingularName: "templateversion",
		Namespaced:   false,
		Kind:         TemplateVersionGroupVersionKind.Kind,
	}
)

type TemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateVersion
}

type TemplateVersionHandlerFunc func(key string, obj *TemplateVersion) error

type TemplateVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*TemplateVersion, err error)
	Get(namespace, name string) (*TemplateVersion, error)
}

type TemplateVersionController interface {
	Informer() cache.SharedIndexInformer
	Lister() TemplateVersionLister
	AddHandler(handler TemplateVersionHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateVersionInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*TemplateVersion) (*TemplateVersion, error)
	Get(name string, opts metav1.GetOptions) (*TemplateVersion, error)
	Update(*TemplateVersion) (*TemplateVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateVersionController
}

type templateVersionLister struct {
	controller *templateVersionController
}

func (l *templateVersionLister) List(namespace string, selector labels.Selector) (ret []*TemplateVersion, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*TemplateVersion))
	})
	return
}

func (l *templateVersionLister) Get(namespace, name string) (*TemplateVersion, error) {
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
			Group:    TemplateVersionGroupVersionKind.Group,
			Resource: "templateVersion",
		}, name)
	}
	return obj.(*TemplateVersion), nil
}

type templateVersionController struct {
	controller.GenericController
}

func (c *templateVersionController) Lister() TemplateVersionLister {
	return &templateVersionLister{
		controller: c,
	}
}

func (c *templateVersionController) AddHandler(handler TemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*TemplateVersion))
	})
}

type templateVersionFactory struct {
}

func (c templateVersionFactory) Object() runtime.Object {
	return &TemplateVersion{}
}

func (c templateVersionFactory) List() runtime.Object {
	return &TemplateVersionList{}
}

func (s *templateVersionClient) Controller() TemplateVersionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.templateVersionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TemplateVersionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &templateVersionController{
		GenericController: genericController,
	}

	s.client.templateVersionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type templateVersionClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   TemplateVersionController
}

func (s *templateVersionClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *templateVersionClient) Create(o *TemplateVersion) (*TemplateVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) Get(name string, opts metav1.GetOptions) (*TemplateVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) Update(o *TemplateVersion) (*TemplateVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateVersionClient) List(opts metav1.ListOptions) (*TemplateVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateVersionList), err
}

func (s *templateVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *templateVersionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}
