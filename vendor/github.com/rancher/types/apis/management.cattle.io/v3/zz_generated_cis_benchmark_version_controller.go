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
	CisBenchmarkVersionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CisBenchmarkVersion",
	}
	CisBenchmarkVersionResource = metav1.APIResource{
		Name:         "cisbenchmarkversions",
		SingularName: "cisbenchmarkversion",
		Namespaced:   true,

		Kind: CisBenchmarkVersionGroupVersionKind.Kind,
	}

	CisBenchmarkVersionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "cisbenchmarkversions",
	}
)

func init() {
	resource.Put(CisBenchmarkVersionGroupVersionResource)
}

func NewCisBenchmarkVersion(namespace, name string, obj CisBenchmarkVersion) *CisBenchmarkVersion {
	obj.APIVersion, obj.Kind = CisBenchmarkVersionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CisBenchmarkVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CisBenchmarkVersion `json:"items"`
}

type CisBenchmarkVersionHandlerFunc func(key string, obj *CisBenchmarkVersion) (runtime.Object, error)

type CisBenchmarkVersionChangeHandlerFunc func(obj *CisBenchmarkVersion) (runtime.Object, error)

type CisBenchmarkVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*CisBenchmarkVersion, err error)
	Get(namespace, name string) (*CisBenchmarkVersion, error)
}

type CisBenchmarkVersionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CisBenchmarkVersionLister
	AddHandler(ctx context.Context, name string, handler CisBenchmarkVersionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CisBenchmarkVersionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CisBenchmarkVersionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CisBenchmarkVersionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CisBenchmarkVersionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CisBenchmarkVersion) (*CisBenchmarkVersion, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CisBenchmarkVersion, error)
	Get(name string, opts metav1.GetOptions) (*CisBenchmarkVersion, error)
	Update(*CisBenchmarkVersion) (*CisBenchmarkVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CisBenchmarkVersionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*CisBenchmarkVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CisBenchmarkVersionController
	AddHandler(ctx context.Context, name string, sync CisBenchmarkVersionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CisBenchmarkVersionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CisBenchmarkVersionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CisBenchmarkVersionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CisBenchmarkVersionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CisBenchmarkVersionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CisBenchmarkVersionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CisBenchmarkVersionLifecycle)
}

type cisBenchmarkVersionLister struct {
	controller *cisBenchmarkVersionController
}

func (l *cisBenchmarkVersionLister) List(namespace string, selector labels.Selector) (ret []*CisBenchmarkVersion, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CisBenchmarkVersion))
	})
	return
}

func (l *cisBenchmarkVersionLister) Get(namespace, name string) (*CisBenchmarkVersion, error) {
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
			Group:    CisBenchmarkVersionGroupVersionKind.Group,
			Resource: "cisBenchmarkVersion",
		}, key)
	}
	return obj.(*CisBenchmarkVersion), nil
}

type cisBenchmarkVersionController struct {
	controller.GenericController
}

func (c *cisBenchmarkVersionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cisBenchmarkVersionController) Lister() CisBenchmarkVersionLister {
	return &cisBenchmarkVersionLister{
		controller: c,
	}
}

func (c *cisBenchmarkVersionController) AddHandler(ctx context.Context, name string, handler CisBenchmarkVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CisBenchmarkVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cisBenchmarkVersionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CisBenchmarkVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CisBenchmarkVersion); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cisBenchmarkVersionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CisBenchmarkVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CisBenchmarkVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cisBenchmarkVersionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CisBenchmarkVersionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CisBenchmarkVersion); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type cisBenchmarkVersionFactory struct {
}

func (c cisBenchmarkVersionFactory) Object() runtime.Object {
	return &CisBenchmarkVersion{}
}

func (c cisBenchmarkVersionFactory) List() runtime.Object {
	return &CisBenchmarkVersionList{}
}

func (s *cisBenchmarkVersionClient) Controller() CisBenchmarkVersionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.cisBenchmarkVersionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CisBenchmarkVersionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &cisBenchmarkVersionController{
		GenericController: genericController,
	}

	s.client.cisBenchmarkVersionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type cisBenchmarkVersionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CisBenchmarkVersionController
}

func (s *cisBenchmarkVersionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cisBenchmarkVersionClient) Create(o *CisBenchmarkVersion) (*CisBenchmarkVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CisBenchmarkVersion), err
}

func (s *cisBenchmarkVersionClient) Get(name string, opts metav1.GetOptions) (*CisBenchmarkVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CisBenchmarkVersion), err
}

func (s *cisBenchmarkVersionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CisBenchmarkVersion, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CisBenchmarkVersion), err
}

func (s *cisBenchmarkVersionClient) Update(o *CisBenchmarkVersion) (*CisBenchmarkVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CisBenchmarkVersion), err
}

func (s *cisBenchmarkVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cisBenchmarkVersionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cisBenchmarkVersionClient) List(opts metav1.ListOptions) (*CisBenchmarkVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CisBenchmarkVersionList), err
}

func (s *cisBenchmarkVersionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*CisBenchmarkVersionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*CisBenchmarkVersionList), err
}

func (s *cisBenchmarkVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cisBenchmarkVersionClient) Patch(o *CisBenchmarkVersion, patchType types.PatchType, data []byte, subresources ...string) (*CisBenchmarkVersion, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*CisBenchmarkVersion), err
}

func (s *cisBenchmarkVersionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cisBenchmarkVersionClient) AddHandler(ctx context.Context, name string, sync CisBenchmarkVersionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cisBenchmarkVersionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CisBenchmarkVersionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cisBenchmarkVersionClient) AddLifecycle(ctx context.Context, name string, lifecycle CisBenchmarkVersionLifecycle) {
	sync := NewCisBenchmarkVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cisBenchmarkVersionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CisBenchmarkVersionLifecycle) {
	sync := NewCisBenchmarkVersionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cisBenchmarkVersionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CisBenchmarkVersionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cisBenchmarkVersionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CisBenchmarkVersionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *cisBenchmarkVersionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CisBenchmarkVersionLifecycle) {
	sync := NewCisBenchmarkVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cisBenchmarkVersionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CisBenchmarkVersionLifecycle) {
	sync := NewCisBenchmarkVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
