package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	CatalogTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CatalogTemplate",
	}
	CatalogTemplateResource = metav1.APIResource{
		Name:         "catalogtemplates",
		SingularName: "catalogtemplate",
		Namespaced:   true,

		Kind: CatalogTemplateGroupVersionKind.Kind,
	}

	CatalogTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "catalogtemplates",
	}
)

func init() {
	resource.Put(CatalogTemplateGroupVersionResource)
}

// Deprecated: use v3.CatalogTemplate instead
type CatalogTemplate = v3.CatalogTemplate

func NewCatalogTemplate(namespace, name string, obj v3.CatalogTemplate) *v3.CatalogTemplate {
	obj.APIVersion, obj.Kind = CatalogTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CatalogTemplateHandlerFunc func(key string, obj *v3.CatalogTemplate) (runtime.Object, error)

type CatalogTemplateChangeHandlerFunc func(obj *v3.CatalogTemplate) (runtime.Object, error)

type CatalogTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.CatalogTemplate, err error)
	Get(namespace, name string) (*v3.CatalogTemplate, error)
}

type CatalogTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CatalogTemplateLister
	AddHandler(ctx context.Context, name string, handler CatalogTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CatalogTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CatalogTemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type CatalogTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.CatalogTemplate) (*v3.CatalogTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.CatalogTemplate, error)
	Get(name string, opts metav1.GetOptions) (*v3.CatalogTemplate, error)
	Update(*v3.CatalogTemplate) (*v3.CatalogTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.CatalogTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CatalogTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CatalogTemplateController
	AddHandler(ctx context.Context, name string, sync CatalogTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogTemplateLifecycle)
}

type catalogTemplateLister struct {
	ns         string
	controller *catalogTemplateController
}

func (l *catalogTemplateLister) List(namespace string, selector labels.Selector) (ret []*v3.CatalogTemplate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.CatalogTemplate))
	})
	return
}

func (l *catalogTemplateLister) Get(namespace, name string) (*v3.CatalogTemplate, error) {
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
			Group:    CatalogTemplateGroupVersionKind.Group,
			Resource: CatalogTemplateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.CatalogTemplate), nil
}

type catalogTemplateController struct {
	ns string
	controller.GenericController
}

func (c *catalogTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *catalogTemplateController) Lister() CatalogTemplateLister {
	return &catalogTemplateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *catalogTemplateController) AddHandler(ctx context.Context, name string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CatalogTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type catalogTemplateFactory struct {
}

func (c catalogTemplateFactory) Object() runtime.Object {
	return &v3.CatalogTemplate{}
}

func (c catalogTemplateFactory) List() runtime.Object {
	return &v3.CatalogTemplateList{}
}

func (s *catalogTemplateClient) Controller() CatalogTemplateController {
	genericController := controller.NewGenericController(s.ns, CatalogTemplateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(CatalogTemplateGroupVersionResource, CatalogTemplateGroupVersionKind.Kind, true))

	return &catalogTemplateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type catalogTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CatalogTemplateController
}

func (s *catalogTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *catalogTemplateClient) Create(o *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.CatalogTemplate), err
}

func (s *catalogTemplateClient) Get(name string, opts metav1.GetOptions) (*v3.CatalogTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.CatalogTemplate), err
}

func (s *catalogTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.CatalogTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.CatalogTemplate), err
}

func (s *catalogTemplateClient) Update(o *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.CatalogTemplate), err
}

func (s *catalogTemplateClient) UpdateStatus(o *v3.CatalogTemplate) (*v3.CatalogTemplate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.CatalogTemplate), err
}

func (s *catalogTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *catalogTemplateClient) List(opts metav1.ListOptions) (*v3.CatalogTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.CatalogTemplateList), err
}

func (s *catalogTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CatalogTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.CatalogTemplateList), err
}

func (s *catalogTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogTemplateClient) Patch(o *v3.CatalogTemplate, patchType types.PatchType, data []byte, subresources ...string) (*v3.CatalogTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.CatalogTemplate), err
}

func (s *catalogTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *catalogTemplateClient) AddHandler(ctx context.Context, name string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *catalogTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogTemplateLifecycle) {
	sync := NewCatalogTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
