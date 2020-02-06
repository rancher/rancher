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
	GlobalDNSGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalDNS",
	}
	GlobalDNSResource = metav1.APIResource{
		Name:         "globaldnses",
		SingularName: "globaldns",
		Namespaced:   true,

		Kind: GlobalDNSGroupVersionKind.Kind,
	}

	GlobalDNSGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globaldnses",
	}
)

func init() {
	resource.Put(GlobalDNSGroupVersionResource)
}

func NewGlobalDNS(namespace, name string, obj GlobalDNS) *GlobalDNS {
	obj.APIVersion, obj.Kind = GlobalDNSGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalDNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalDNS `json:"items"`
}

type GlobalDNSHandlerFunc func(key string, obj *GlobalDNS) (runtime.Object, error)

type GlobalDNSChangeHandlerFunc func(obj *GlobalDNS) (runtime.Object, error)

type GlobalDNSLister interface {
	List(namespace string, selector labels.Selector) (ret []*GlobalDNS, err error)
	Get(namespace, name string) (*GlobalDNS, error)
}

type GlobalDNSController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalDNSLister
	AddHandler(ctx context.Context, name string, handler GlobalDNSHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDNSHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalDNSHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalDNSHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GlobalDNSInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*GlobalDNS) (*GlobalDNS, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalDNS, error)
	Get(name string, opts metav1.GetOptions) (*GlobalDNS, error)
	Update(*GlobalDNS) (*GlobalDNS, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GlobalDNSList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*GlobalDNSList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalDNSController
	AddHandler(ctx context.Context, name string, sync GlobalDNSHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDNSHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalDNSLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDNSLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDNSHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDNSHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDNSLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDNSLifecycle)
}

type globalDnsLister struct {
	controller *globalDnsController
}

func (l *globalDnsLister) List(namespace string, selector labels.Selector) (ret []*GlobalDNS, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GlobalDNS))
	})
	return
}

func (l *globalDnsLister) Get(namespace, name string) (*GlobalDNS, error) {
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
			Group:    GlobalDNSGroupVersionKind.Group,
			Resource: "globalDns",
		}, key)
	}
	return obj.(*GlobalDNS), nil
}

type globalDnsController struct {
	controller.GenericController
}

func (c *globalDnsController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalDnsController) Lister() GlobalDNSLister {
	return &globalDnsLister{
		controller: c,
	}
}

func (c *globalDnsController) AddHandler(ctx context.Context, name string, handler GlobalDNSHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalDNS); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalDNSHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalDNS); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalDNSHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalDNS); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalDnsController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalDNSHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalDNS); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalDnsFactory struct {
}

func (c globalDnsFactory) Object() runtime.Object {
	return &GlobalDNS{}
}

func (c globalDnsFactory) List() runtime.Object {
	return &GlobalDNSList{}
}

func (s *globalDnsClient) Controller() GlobalDNSController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.globalDnsControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GlobalDNSGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &globalDnsController{
		GenericController: genericController,
	}

	s.client.globalDnsControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type globalDnsClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalDNSController
}

func (s *globalDnsClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalDnsClient) Create(o *GlobalDNS) (*GlobalDNS, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) Get(name string, opts metav1.GetOptions) (*GlobalDNS, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalDNS, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) Update(o *GlobalDNS) (*GlobalDNS, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalDnsClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalDnsClient) List(opts metav1.ListOptions) (*GlobalDNSList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GlobalDNSList), err
}

func (s *globalDnsClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*GlobalDNSList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*GlobalDNSList), err
}

func (s *globalDnsClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalDnsClient) Patch(o *GlobalDNS, patchType types.PatchType, data []byte, subresources ...string) (*GlobalDNS, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*GlobalDNS), err
}

func (s *globalDnsClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalDnsClient) AddHandler(ctx context.Context, name string, sync GlobalDNSHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalDNSHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalDNSLifecycle) {
	sync := NewGlobalDNSLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalDnsClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalDNSLifecycle) {
	sync := NewGlobalDNSLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalDnsClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalDNSHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalDNSHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalDNSLifecycle) {
	sync := NewGlobalDNSLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalDnsClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalDNSLifecycle) {
	sync := NewGlobalDNSLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
