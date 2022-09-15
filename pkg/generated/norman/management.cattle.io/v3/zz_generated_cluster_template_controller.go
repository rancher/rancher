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
	ClusterTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterTemplate",
	}
	ClusterTemplateResource = metav1.APIResource{
		Name:         "clustertemplates",
		SingularName: "clustertemplate",
		Namespaced:   true,

		Kind: ClusterTemplateGroupVersionKind.Kind,
	}

	ClusterTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clustertemplates",
	}
)

func init() {
	resource.Put(ClusterTemplateGroupVersionResource)
}

// Deprecated: use v3.ClusterTemplate instead
type ClusterTemplate = v3.ClusterTemplate

func NewClusterTemplate(namespace, name string, obj v3.ClusterTemplate) *v3.ClusterTemplate {
	obj.APIVersion, obj.Kind = ClusterTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterTemplateHandlerFunc func(key string, obj *v3.ClusterTemplate) (runtime.Object, error)

type ClusterTemplateChangeHandlerFunc func(obj *v3.ClusterTemplate) (runtime.Object, error)

type ClusterTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ClusterTemplate, err error)
	Get(namespace, name string) (*v3.ClusterTemplate, error)
}

type ClusterTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterTemplateLister
	AddHandler(ctx context.Context, name string, handler ClusterTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterTemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ClusterTemplate) (*v3.ClusterTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterTemplate, error)
	Get(name string, opts metav1.GetOptions) (*v3.ClusterTemplate, error)
	Update(*v3.ClusterTemplate) (*v3.ClusterTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ClusterTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterTemplateController
	AddHandler(ctx context.Context, name string, sync ClusterTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterTemplateLifecycle)
}

type clusterTemplateLister struct {
	ns         string
	controller *clusterTemplateController
}

func (l *clusterTemplateLister) List(namespace string, selector labels.Selector) (ret []*v3.ClusterTemplate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ClusterTemplate))
	})
	return
}

func (l *clusterTemplateLister) Get(namespace, name string) (*v3.ClusterTemplate, error) {
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
			Group:    ClusterTemplateGroupVersionKind.Group,
			Resource: ClusterTemplateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ClusterTemplate), nil
}

type clusterTemplateController struct {
	ns string
	controller.GenericController
}

func (c *clusterTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterTemplateController) Lister() ClusterTemplateLister {
	return &clusterTemplateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterTemplateController) AddHandler(ctx context.Context, name string, handler ClusterTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterTemplateFactory struct {
}

func (c clusterTemplateFactory) Object() runtime.Object {
	return &v3.ClusterTemplate{}
}

func (c clusterTemplateFactory) List() runtime.Object {
	return &v3.ClusterTemplateList{}
}

func (s *clusterTemplateClient) Controller() ClusterTemplateController {
	genericController := controller.NewGenericController(s.ns, ClusterTemplateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterTemplateGroupVersionResource, ClusterTemplateGroupVersionKind.Kind, true))

	return &clusterTemplateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterTemplateController
}

func (s *clusterTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterTemplateClient) Create(o *v3.ClusterTemplate) (*v3.ClusterTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ClusterTemplate), err
}

func (s *clusterTemplateClient) Get(name string, opts metav1.GetOptions) (*v3.ClusterTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ClusterTemplate), err
}

func (s *clusterTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ClusterTemplate), err
}

func (s *clusterTemplateClient) Update(o *v3.ClusterTemplate) (*v3.ClusterTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ClusterTemplate), err
}

func (s *clusterTemplateClient) UpdateStatus(o *v3.ClusterTemplate) (*v3.ClusterTemplate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ClusterTemplate), err
}

func (s *clusterTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterTemplateClient) List(opts metav1.ListOptions) (*v3.ClusterTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ClusterTemplateList), err
}

func (s *clusterTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ClusterTemplateList), err
}

func (s *clusterTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterTemplateClient) Patch(o *v3.ClusterTemplate, patchType types.PatchType, data []byte, subresources ...string) (*v3.ClusterTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ClusterTemplate), err
}

func (s *clusterTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterTemplateClient) AddHandler(ctx context.Context, name string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
