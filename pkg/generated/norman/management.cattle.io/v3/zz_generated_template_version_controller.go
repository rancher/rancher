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
	TemplateVersionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "TemplateVersion",
	}
	TemplateVersionResource = metav1.APIResource{
		Name:         "templateversions",
		SingularName: "templateversion",
		Namespaced:   false,
		Kind:         TemplateVersionGroupVersionKind.Kind,
	}

	TemplateVersionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "templateversions",
	}
)

func init() {
	resource.Put(TemplateVersionGroupVersionResource)
}

// Deprecated: use v3.TemplateVersion instead
type TemplateVersion = v3.TemplateVersion

func NewTemplateVersion(namespace, name string, obj v3.TemplateVersion) *v3.TemplateVersion {
	obj.APIVersion, obj.Kind = TemplateVersionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TemplateVersionHandlerFunc func(key string, obj *v3.TemplateVersion) (runtime.Object, error)

type TemplateVersionChangeHandlerFunc func(obj *v3.TemplateVersion) (runtime.Object, error)

type TemplateVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.TemplateVersion, err error)
	Get(namespace, name string) (*v3.TemplateVersion, error)
}

type TemplateVersionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TemplateVersionLister
	AddHandler(ctx context.Context, name string, handler TemplateVersionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateVersionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TemplateVersionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler TemplateVersionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type TemplateVersionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.TemplateVersion) (*v3.TemplateVersion, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.TemplateVersion, error)
	Get(name string, opts metav1.GetOptions) (*v3.TemplateVersion, error)
	Update(*v3.TemplateVersion) (*v3.TemplateVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.TemplateVersionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.TemplateVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateVersionController
	AddHandler(ctx context.Context, name string, sync TemplateVersionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateVersionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TemplateVersionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateVersionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateVersionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateVersionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateVersionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateVersionLifecycle)
}

type templateVersionLister struct {
	ns         string
	controller *templateVersionController
}

func (l *templateVersionLister) List(namespace string, selector labels.Selector) (ret []*v3.TemplateVersion, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.TemplateVersion))
	})
	return
}

func (l *templateVersionLister) Get(namespace, name string) (*v3.TemplateVersion, error) {
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
			Resource: TemplateVersionGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.TemplateVersion), nil
}

type templateVersionController struct {
	ns string
	controller.GenericController
}

func (c *templateVersionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *templateVersionController) Lister() TemplateVersionLister {
	return &templateVersionLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *templateVersionController) AddHandler(ctx context.Context, name string, handler TemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.TemplateVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateVersionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler TemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.TemplateVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateVersionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.TemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateVersionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler TemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.TemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type templateVersionFactory struct {
}

func (c templateVersionFactory) Object() runtime.Object {
	return &v3.TemplateVersion{}
}

func (c templateVersionFactory) List() runtime.Object {
	return &v3.TemplateVersionList{}
}

func (s *templateVersionClient) Controller() TemplateVersionController {
	genericController := controller.NewGenericController(s.ns, TemplateVersionGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(TemplateVersionGroupVersionResource, TemplateVersionGroupVersionKind.Kind, false))

	return &templateVersionController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type templateVersionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TemplateVersionController
}

func (s *templateVersionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *templateVersionClient) Create(o *v3.TemplateVersion) (*v3.TemplateVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.TemplateVersion), err
}

func (s *templateVersionClient) Get(name string, opts metav1.GetOptions) (*v3.TemplateVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.TemplateVersion), err
}

func (s *templateVersionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.TemplateVersion, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.TemplateVersion), err
}

func (s *templateVersionClient) Update(o *v3.TemplateVersion) (*v3.TemplateVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.TemplateVersion), err
}

func (s *templateVersionClient) UpdateStatus(o *v3.TemplateVersion) (*v3.TemplateVersion, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.TemplateVersion), err
}

func (s *templateVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateVersionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *templateVersionClient) List(opts metav1.ListOptions) (*v3.TemplateVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.TemplateVersionList), err
}

func (s *templateVersionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.TemplateVersionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.TemplateVersionList), err
}

func (s *templateVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateVersionClient) Patch(o *v3.TemplateVersion, patchType types.PatchType, data []byte, subresources ...string) (*v3.TemplateVersion, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.TemplateVersion), err
}

func (s *templateVersionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateVersionClient) AddHandler(ctx context.Context, name string, sync TemplateVersionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateVersionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateVersionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateVersionClient) AddLifecycle(ctx context.Context, name string, lifecycle TemplateVersionLifecycle) {
	sync := NewTemplateVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateVersionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateVersionLifecycle) {
	sync := NewTemplateVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateVersionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateVersionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateVersionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateVersionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *templateVersionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateVersionLifecycle) {
	sync := NewTemplateVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateVersionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateVersionLifecycle) {
	sync := NewTemplateVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
