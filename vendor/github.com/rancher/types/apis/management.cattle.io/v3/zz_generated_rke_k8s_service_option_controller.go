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
	RKEK8sServiceOptionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RKEK8sServiceOption",
	}
	RKEK8sServiceOptionResource = metav1.APIResource{
		Name:         "rkek8sserviceoptions",
		SingularName: "rkek8sserviceoption",
		Namespaced:   true,

		Kind: RKEK8sServiceOptionGroupVersionKind.Kind,
	}

	RKEK8sServiceOptionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rkek8sserviceoptions",
	}
)

func init() {
	resource.Put(RKEK8sServiceOptionGroupVersionResource)
}

func NewRKEK8sServiceOption(namespace, name string, obj RKEK8sServiceOption) *RKEK8sServiceOption {
	obj.APIVersion, obj.Kind = RKEK8sServiceOptionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RKEK8sServiceOptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RKEK8sServiceOption `json:"items"`
}

type RKEK8sServiceOptionHandlerFunc func(key string, obj *RKEK8sServiceOption) (runtime.Object, error)

type RKEK8sServiceOptionChangeHandlerFunc func(obj *RKEK8sServiceOption) (runtime.Object, error)

type RKEK8sServiceOptionLister interface {
	List(namespace string, selector labels.Selector) (ret []*RKEK8sServiceOption, err error)
	Get(namespace, name string) (*RKEK8sServiceOption, error)
}

type RKEK8sServiceOptionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RKEK8sServiceOptionLister
	AddHandler(ctx context.Context, name string, handler RKEK8sServiceOptionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sServiceOptionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RKEK8sServiceOptionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RKEK8sServiceOptionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RKEK8sServiceOptionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*RKEK8sServiceOption) (*RKEK8sServiceOption, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEK8sServiceOption, error)
	Get(name string, opts metav1.GetOptions) (*RKEK8sServiceOption, error)
	Update(*RKEK8sServiceOption) (*RKEK8sServiceOption, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RKEK8sServiceOptionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*RKEK8sServiceOptionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RKEK8sServiceOptionController
	AddHandler(ctx context.Context, name string, sync RKEK8sServiceOptionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sServiceOptionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RKEK8sServiceOptionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEK8sServiceOptionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEK8sServiceOptionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEK8sServiceOptionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEK8sServiceOptionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEK8sServiceOptionLifecycle)
}

type rkeK8sServiceOptionLister struct {
	controller *rkeK8sServiceOptionController
}

func (l *rkeK8sServiceOptionLister) List(namespace string, selector labels.Selector) (ret []*RKEK8sServiceOption, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*RKEK8sServiceOption))
	})
	return
}

func (l *rkeK8sServiceOptionLister) Get(namespace, name string) (*RKEK8sServiceOption, error) {
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
			Group:    RKEK8sServiceOptionGroupVersionKind.Group,
			Resource: "rkeK8sServiceOption",
		}, key)
	}
	return obj.(*RKEK8sServiceOption), nil
}

type rkeK8sServiceOptionController struct {
	controller.GenericController
}

func (c *rkeK8sServiceOptionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rkeK8sServiceOptionController) Lister() RKEK8sServiceOptionLister {
	return &rkeK8sServiceOptionLister{
		controller: c,
	}
}

func (c *rkeK8sServiceOptionController) AddHandler(ctx context.Context, name string, handler RKEK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sServiceOption); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sServiceOptionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RKEK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sServiceOption); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sServiceOptionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RKEK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sServiceOption); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sServiceOptionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RKEK8sServiceOptionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sServiceOption); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rkeK8sServiceOptionFactory struct {
}

func (c rkeK8sServiceOptionFactory) Object() runtime.Object {
	return &RKEK8sServiceOption{}
}

func (c rkeK8sServiceOptionFactory) List() runtime.Object {
	return &RKEK8sServiceOptionList{}
}

func (s *rkeK8sServiceOptionClient) Controller() RKEK8sServiceOptionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.rkeK8sServiceOptionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RKEK8sServiceOptionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &rkeK8sServiceOptionController{
		GenericController: genericController,
	}

	s.client.rkeK8sServiceOptionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type rkeK8sServiceOptionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RKEK8sServiceOptionController
}

func (s *rkeK8sServiceOptionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rkeK8sServiceOptionClient) Create(o *RKEK8sServiceOption) (*RKEK8sServiceOption, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*RKEK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) Get(name string, opts metav1.GetOptions) (*RKEK8sServiceOption, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*RKEK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEK8sServiceOption, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*RKEK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) Update(o *RKEK8sServiceOption) (*RKEK8sServiceOption, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*RKEK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rkeK8sServiceOptionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rkeK8sServiceOptionClient) List(opts metav1.ListOptions) (*RKEK8sServiceOptionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RKEK8sServiceOptionList), err
}

func (s *rkeK8sServiceOptionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*RKEK8sServiceOptionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*RKEK8sServiceOptionList), err
}

func (s *rkeK8sServiceOptionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rkeK8sServiceOptionClient) Patch(o *RKEK8sServiceOption, patchType types.PatchType, data []byte, subresources ...string) (*RKEK8sServiceOption, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*RKEK8sServiceOption), err
}

func (s *rkeK8sServiceOptionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rkeK8sServiceOptionClient) AddHandler(ctx context.Context, name string, sync RKEK8sServiceOptionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sServiceOptionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddLifecycle(ctx context.Context, name string, lifecycle RKEK8sServiceOptionLifecycle) {
	sync := NewRKEK8sServiceOptionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEK8sServiceOptionLifecycle) {
	sync := NewRKEK8sServiceOptionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEK8sServiceOptionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEK8sServiceOptionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEK8sServiceOptionLifecycle) {
	sync := NewRKEK8sServiceOptionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sServiceOptionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEK8sServiceOptionLifecycle) {
	sync := NewRKEK8sServiceOptionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
