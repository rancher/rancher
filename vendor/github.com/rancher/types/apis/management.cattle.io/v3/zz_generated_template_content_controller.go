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

	TemplateContentGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "templatecontents",
	}
)

func init() {
	resource.Put(TemplateContentGroupVersionResource)
}

func NewTemplateContent(namespace, name string, obj TemplateContent) *TemplateContent {
	obj.APIVersion, obj.Kind = TemplateContentGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TemplateContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateContent `json:"items"`
}

type TemplateContentHandlerFunc func(key string, obj *TemplateContent) (runtime.Object, error)

type TemplateContentChangeHandlerFunc func(obj *TemplateContent) (runtime.Object, error)

type TemplateContentLister interface {
	List(namespace string, selector labels.Selector) (ret []*TemplateContent, err error)
	Get(namespace, name string) (*TemplateContent, error)
}

type TemplateContentController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TemplateContentLister
	AddHandler(ctx context.Context, name string, handler TemplateContentHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateContentHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TemplateContentHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler TemplateContentHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
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
	ListNamespaced(namespace string, opts metav1.ListOptions) (*TemplateContentList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateContentController
	AddHandler(ctx context.Context, name string, sync TemplateContentHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateContentHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TemplateContentLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateContentLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateContentHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateContentHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateContentLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateContentLifecycle)
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

func (c *templateContentController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *templateContentController) Lister() TemplateContentLister {
	return &templateContentLister{
		controller: c,
	}
}

func (c *templateContentController) AddHandler(ctx context.Context, name string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*TemplateContent); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateContentController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*TemplateContent); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateContentController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*TemplateContent); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateContentController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*TemplateContent); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
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

func (s *templateContentClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*TemplateContentList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*TemplateContentList), err
}

func (s *templateContentClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateContentClient) Patch(o *TemplateContent, patchType types.PatchType, data []byte, subresources ...string) (*TemplateContent, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateContentClient) AddHandler(ctx context.Context, name string, sync TemplateContentHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateContentClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateContentHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateContentClient) AddLifecycle(ctx context.Context, name string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateContentClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateContentClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateContentHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateContentClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateContentHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *templateContentClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateContentClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
