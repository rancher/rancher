package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	TemplateContentGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "TemplateContent",
	}
	TemplateContentResource = metav1.APIResource{
		Name:         "templatecontents",
		SingularName: "templatecontent",
		Namespaced:   false,
		Kind:         TemplateContentGroupVersionKind.Kind,
	}
)

type TemplateContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateContent
}

type TemplateContentHandlerFunc func(key string, obj *TemplateContent) error

type TemplateContentLister interface {
	List(namespace string, selector labels.Selector) (ret []*TemplateContent, err error)
	Get(namespace, name string) (*TemplateContent, error)
}

type TemplateContentController interface {
	Informer() cache.SharedIndexInformer
	Lister() TemplateContentLister
	AddHandler(name string, handler TemplateContentHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler TemplateContentHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateContentInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*TemplateContent) (*TemplateContent, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*TemplateContent, error)
	Get(name string, opts metav1.GetOptions) (*TemplateContent, error)
	Update(*TemplateContent) (*TemplateContent, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateContentList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateContentController
	AddHandler(name string, sync TemplateContentHandlerFunc)
	AddLifecycle(name string, lifecycle TemplateContentLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync TemplateContentHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle TemplateContentLifecycle)
}

type templateContentLister struct {
	controller *templateContentController
}

func (l *templateContentLister) List(namespace string, selector labels.Selector) (ret []*TemplateContent, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*TemplateContent))
	})
	return
}

func (l *templateContentLister) Get(namespace, name string) (*TemplateContent, error) {
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
			Group:    TemplateContentGroupVersionKind.Group,
			Resource: "templateContent",
		}, key)
	}
	return obj.(*TemplateContent), nil
}

type templateContentController struct {
	controller.GenericController
}

func (c *templateContentController) Lister() TemplateContentLister {
	return &templateContentLister{
		controller: c,
	}
}

func (c *templateContentController) AddHandler(name string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*TemplateContent))
	})
}

func (c *templateContentController) AddClusterScopedHandler(name, cluster string, handler TemplateContentHandlerFunc) {
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

		return handler(key, obj.(*TemplateContent))
	})
}

type templateContentFactory struct {
}

func (c templateContentFactory) Object() runtime.Object {
	return &TemplateContent{}
}

func (c templateContentFactory) List() runtime.Object {
	return &TemplateContentList{}
}

func (s *templateContentClient) Controller() TemplateContentController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.templateContentControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TemplateContentGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &templateContentController{
		GenericController: genericController,
	}

	s.client.templateContentControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type templateContentClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TemplateContentController
}

func (s *templateContentClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *templateContentClient) Create(o *TemplateContent) (*TemplateContent, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) Get(name string, opts metav1.GetOptions) (*TemplateContent, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*TemplateContent, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) Update(o *TemplateContent) (*TemplateContent, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateContentClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *templateContentClient) List(opts metav1.ListOptions) (*TemplateContentList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateContentList), err
}

func (s *templateContentClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateContentClient) Patch(o *TemplateContent, data []byte, subresources ...string) (*TemplateContent, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateContentClient) AddHandler(name string, sync TemplateContentHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *templateContentClient) AddLifecycle(name string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *templateContentClient) AddClusterScopedHandler(name, clusterName string, sync TemplateContentHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *templateContentClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
