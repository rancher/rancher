package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

	TemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "templates",
	}
)

func init() {
	resource.Put(TemplateGroupVersionResource)
}

func NewTemplate(namespace, name string, obj Template) *Template {
	obj.APIVersion, obj.Kind = TemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

type TemplateHandlerFunc func(key string, obj *Template) (runtime.Object, error)

type TemplateChangeHandlerFunc func(obj *Template) (runtime.Object, error)

type TemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*Template, err error)
	Get(namespace, name string) (*Template, error)
}

type TemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TemplateLister
	AddHandler(ctx context.Context, name string, handler TemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler TemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Template) (*Template, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Template, error)
	Get(name string, opts metav1.GetOptions) (*Template, error)
	Update(*Template) (*Template, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*TemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateController
	AddHandler(ctx context.Context, name string, sync TemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateLifecycle)
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
		}, key)
	}
	return obj.(*Template), nil
}

type templateController struct {
	controller.GenericController
}

func (c *templateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *templateController) Lister() TemplateLister {
	return &templateLister{
		controller: c,
	}
}

func (c *templateController) AddHandler(ctx context.Context, name string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
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
	objectClient *objectclient.ObjectClient
	controller   TemplateController
}

func (s *templateClient) ObjectClient() *objectclient.ObjectClient {
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

func (s *templateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Template, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Template), err
}

func (s *templateClient) Update(o *Template) (*Template, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Template), err
}

func (s *templateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *templateClient) List(opts metav1.ListOptions) (*TemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateList), err
}

func (s *templateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*TemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*TemplateList), err
}

func (s *templateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateClient) Patch(o *Template, patchType types.PatchType, data []byte, subresources ...string) (*Template, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Template), err
}

func (s *templateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateClient) AddHandler(ctx context.Context, name string, sync TemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateClient) AddLifecycle(ctx context.Context, name string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *templateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
