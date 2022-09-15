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
	CatalogTemplateVersionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CatalogTemplateVersion",
	}
	CatalogTemplateVersionResource = metav1.APIResource{
		Name:         "catalogtemplateversions",
		SingularName: "catalogtemplateversion",
		Namespaced:   true,

		Kind: CatalogTemplateVersionGroupVersionKind.Kind,
	}

	CatalogTemplateVersionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "catalogtemplateversions",
	}
)

func init() {
	resource.Put(CatalogTemplateVersionGroupVersionResource)
}

// Deprecated: use v3.CatalogTemplateVersion instead
type CatalogTemplateVersion = v3.CatalogTemplateVersion

func NewCatalogTemplateVersion(namespace, name string, obj v3.CatalogTemplateVersion) *v3.CatalogTemplateVersion {
	obj.APIVersion, obj.Kind = CatalogTemplateVersionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CatalogTemplateVersionHandlerFunc func(key string, obj *v3.CatalogTemplateVersion) (runtime.Object, error)

type CatalogTemplateVersionChangeHandlerFunc func(obj *v3.CatalogTemplateVersion) (runtime.Object, error)

type CatalogTemplateVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.CatalogTemplateVersion, err error)
	Get(namespace, name string) (*v3.CatalogTemplateVersion, error)
}

type CatalogTemplateVersionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CatalogTemplateVersionLister
	AddHandler(ctx context.Context, name string, handler CatalogTemplateVersionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateVersionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CatalogTemplateVersionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CatalogTemplateVersionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type CatalogTemplateVersionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.CatalogTemplateVersion) (*v3.CatalogTemplateVersion, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.CatalogTemplateVersion, error)
	Get(name string, opts metav1.GetOptions) (*v3.CatalogTemplateVersion, error)
	Update(*v3.CatalogTemplateVersion) (*v3.CatalogTemplateVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.CatalogTemplateVersionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CatalogTemplateVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CatalogTemplateVersionController
	AddHandler(ctx context.Context, name string, sync CatalogTemplateVersionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateVersionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateVersionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogTemplateVersionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateVersionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogTemplateVersionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateVersionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogTemplateVersionLifecycle)
}

type catalogTemplateVersionLister struct {
	ns         string
	controller *catalogTemplateVersionController
}

func (l *catalogTemplateVersionLister) List(namespace string, selector labels.Selector) (ret []*v3.CatalogTemplateVersion, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.CatalogTemplateVersion))
	})
	return
}

func (l *catalogTemplateVersionLister) Get(namespace, name string) (*v3.CatalogTemplateVersion, error) {
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
			Group:    CatalogTemplateVersionGroupVersionKind.Group,
			Resource: CatalogTemplateVersionGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.CatalogTemplateVersion), nil
}

type catalogTemplateVersionController struct {
	ns string
	controller.GenericController
}

func (c *catalogTemplateVersionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *catalogTemplateVersionController) Lister() CatalogTemplateVersionLister {
	return &catalogTemplateVersionLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *catalogTemplateVersionController) AddHandler(ctx context.Context, name string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplateVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateVersionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplateVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateVersionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *catalogTemplateVersionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CatalogTemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type catalogTemplateVersionFactory struct {
}

func (c catalogTemplateVersionFactory) Object() runtime.Object {
	return &v3.CatalogTemplateVersion{}
}

func (c catalogTemplateVersionFactory) List() runtime.Object {
	return &v3.CatalogTemplateVersionList{}
}

func (s *catalogTemplateVersionClient) Controller() CatalogTemplateVersionController {
	genericController := controller.NewGenericController(s.ns, CatalogTemplateVersionGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(CatalogTemplateVersionGroupVersionResource, CatalogTemplateVersionGroupVersionKind.Kind, true))

	return &catalogTemplateVersionController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type catalogTemplateVersionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CatalogTemplateVersionController
}

func (s *catalogTemplateVersionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *catalogTemplateVersionClient) Create(o *v3.CatalogTemplateVersion) (*v3.CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Get(name string, opts metav1.GetOptions) (*v3.CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.CatalogTemplateVersion, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Update(o *v3.CatalogTemplateVersion) (*v3.CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) UpdateStatus(o *v3.CatalogTemplateVersion) (*v3.CatalogTemplateVersion, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogTemplateVersionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *catalogTemplateVersionClient) List(opts metav1.ListOptions) (*v3.CatalogTemplateVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.CatalogTemplateVersionList), err
}

func (s *catalogTemplateVersionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CatalogTemplateVersionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.CatalogTemplateVersionList), err
}

func (s *catalogTemplateVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogTemplateVersionClient) Patch(o *v3.CatalogTemplateVersion, patchType types.PatchType, data []byte, subresources ...string) (*v3.CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *catalogTemplateVersionClient) AddHandler(ctx context.Context, name string, sync CatalogTemplateVersionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateVersionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CatalogTemplateVersionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogTemplateVersionClient) AddLifecycle(ctx context.Context, name string, lifecycle CatalogTemplateVersionLifecycle) {
	sync := NewCatalogTemplateVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *catalogTemplateVersionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CatalogTemplateVersionLifecycle) {
	sync := NewCatalogTemplateVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *catalogTemplateVersionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CatalogTemplateVersionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateVersionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CatalogTemplateVersionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *catalogTemplateVersionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CatalogTemplateVersionLifecycle) {
	sync := NewCatalogTemplateVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *catalogTemplateVersionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CatalogTemplateVersionLifecycle) {
	sync := NewCatalogTemplateVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
