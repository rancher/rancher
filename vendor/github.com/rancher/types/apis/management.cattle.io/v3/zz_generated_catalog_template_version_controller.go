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

func NewCatalogTemplateVersion(namespace, name string, obj CatalogTemplateVersion) *CatalogTemplateVersion {
	obj.APIVersion, obj.Kind = CatalogTemplateVersionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CatalogTemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CatalogTemplateVersion `json:"items"`
}

type CatalogTemplateVersionHandlerFunc func(key string, obj *CatalogTemplateVersion) (runtime.Object, error)

type CatalogTemplateVersionChangeHandlerFunc func(obj *CatalogTemplateVersion) (runtime.Object, error)

type CatalogTemplateVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*CatalogTemplateVersion, err error)
	Get(namespace, name string) (*CatalogTemplateVersion, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CatalogTemplateVersionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error)
	Get(name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error)
	Update(*CatalogTemplateVersion) (*CatalogTemplateVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CatalogTemplateVersionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*CatalogTemplateVersionList, error)
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
	controller *catalogTemplateVersionController
}

func (l *catalogTemplateVersionLister) List(namespace string, selector labels.Selector) (ret []*CatalogTemplateVersion, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CatalogTemplateVersion))
	})
	return
}

func (l *catalogTemplateVersionLister) Get(namespace, name string) (*CatalogTemplateVersion, error) {
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
			Resource: "catalogTemplateVersion",
		}, key)
	}
	return obj.(*CatalogTemplateVersion), nil
}

type catalogTemplateVersionController struct {
	controller.GenericController
}

func (c *catalogTemplateVersionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *catalogTemplateVersionController) Lister() CatalogTemplateVersionLister {
	return &catalogTemplateVersionLister{
		controller: c,
	}
}

func (c *catalogTemplateVersionController) AddHandler(ctx context.Context, name string, handler CatalogTemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CatalogTemplateVersion); ok {
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
		} else if v, ok := obj.(*CatalogTemplateVersion); ok {
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
		} else if v, ok := obj.(*CatalogTemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*CatalogTemplateVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type catalogTemplateVersionFactory struct {
}

func (c catalogTemplateVersionFactory) Object() runtime.Object {
	return &CatalogTemplateVersion{}
}

func (c catalogTemplateVersionFactory) List() runtime.Object {
	return &CatalogTemplateVersionList{}
}

func (s *catalogTemplateVersionClient) Controller() CatalogTemplateVersionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.catalogTemplateVersionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CatalogTemplateVersionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &catalogTemplateVersionController{
		GenericController: genericController,
	}

	s.client.catalogTemplateVersionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *catalogTemplateVersionClient) Create(o *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Get(name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Update(o *CatalogTemplateVersion) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CatalogTemplateVersion), err
}

func (s *catalogTemplateVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *catalogTemplateVersionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *catalogTemplateVersionClient) List(opts metav1.ListOptions) (*CatalogTemplateVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CatalogTemplateVersionList), err
}

func (s *catalogTemplateVersionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*CatalogTemplateVersionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*CatalogTemplateVersionList), err
}

func (s *catalogTemplateVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *catalogTemplateVersionClient) Patch(o *CatalogTemplateVersion, patchType types.PatchType, data []byte, subresources ...string) (*CatalogTemplateVersion, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*CatalogTemplateVersion), err
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
