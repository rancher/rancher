package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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
	PipelineSettingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PipelineSetting",
	}
	PipelineSettingResource = metav1.APIResource{
		Name:         "pipelinesettings",
		SingularName: "pipelinesetting",
		Namespaced:   true,

		Kind: PipelineSettingGroupVersionKind.Kind,
	}

	PipelineSettingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "pipelinesettings",
	}
)

func init() {
	resource.Put(PipelineSettingGroupVersionResource)
}

// Deprecated use v3.PipelineSetting instead
type PipelineSetting = v3.PipelineSetting

func NewPipelineSetting(namespace, name string, obj v3.PipelineSetting) *v3.PipelineSetting {
	obj.APIVersion, obj.Kind = PipelineSettingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PipelineSettingHandlerFunc func(key string, obj *v3.PipelineSetting) (runtime.Object, error)

type PipelineSettingChangeHandlerFunc func(obj *v3.PipelineSetting) (runtime.Object, error)

type PipelineSettingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.PipelineSetting, err error)
	Get(namespace, name string) (*v3.PipelineSetting, error)
}

type PipelineSettingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PipelineSettingLister
	AddHandler(ctx context.Context, name string, handler PipelineSettingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineSettingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PipelineSettingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PipelineSettingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PipelineSettingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.PipelineSetting) (*v3.PipelineSetting, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PipelineSetting, error)
	Get(name string, opts metav1.GetOptions) (*v3.PipelineSetting, error)
	Update(*v3.PipelineSetting) (*v3.PipelineSetting, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PipelineSettingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PipelineSettingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PipelineSettingController
	AddHandler(ctx context.Context, name string, sync PipelineSettingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineSettingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PipelineSettingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineSettingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineSettingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineSettingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineSettingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineSettingLifecycle)
}

type pipelineSettingLister struct {
	controller *pipelineSettingController
}

func (l *pipelineSettingLister) List(namespace string, selector labels.Selector) (ret []*v3.PipelineSetting, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.PipelineSetting))
	})
	return
}

func (l *pipelineSettingLister) Get(namespace, name string) (*v3.PipelineSetting, error) {
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
			Group:    PipelineSettingGroupVersionKind.Group,
			Resource: PipelineSettingGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.PipelineSetting), nil
}

type pipelineSettingController struct {
	controller.GenericController
}

func (c *pipelineSettingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *pipelineSettingController) Lister() PipelineSettingLister {
	return &pipelineSettingLister{
		controller: c,
	}
}

func (c *pipelineSettingController) AddHandler(ctx context.Context, name string, handler PipelineSettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PipelineSetting); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineSettingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PipelineSettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PipelineSetting); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineSettingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PipelineSettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PipelineSetting); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineSettingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PipelineSettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PipelineSetting); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type pipelineSettingFactory struct {
}

func (c pipelineSettingFactory) Object() runtime.Object {
	return &v3.PipelineSetting{}
}

func (c pipelineSettingFactory) List() runtime.Object {
	return &v3.PipelineSettingList{}
}

func (s *pipelineSettingClient) Controller() PipelineSettingController {
	genericController := controller.NewGenericController(PipelineSettingGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PipelineSettingGroupVersionResource, PipelineSettingGroupVersionKind.Kind, true))

	return &pipelineSettingController{
		GenericController: genericController,
	}
}

type pipelineSettingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PipelineSettingController
}

func (s *pipelineSettingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *pipelineSettingClient) Create(o *v3.PipelineSetting) (*v3.PipelineSetting, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.PipelineSetting), err
}

func (s *pipelineSettingClient) Get(name string, opts metav1.GetOptions) (*v3.PipelineSetting, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.PipelineSetting), err
}

func (s *pipelineSettingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PipelineSetting, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.PipelineSetting), err
}

func (s *pipelineSettingClient) Update(o *v3.PipelineSetting) (*v3.PipelineSetting, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.PipelineSetting), err
}

func (s *pipelineSettingClient) UpdateStatus(o *v3.PipelineSetting) (*v3.PipelineSetting, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.PipelineSetting), err
}

func (s *pipelineSettingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineSettingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineSettingClient) List(opts metav1.ListOptions) (*v3.PipelineSettingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PipelineSettingList), err
}

func (s *pipelineSettingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PipelineSettingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PipelineSettingList), err
}

func (s *pipelineSettingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineSettingClient) Patch(o *v3.PipelineSetting, patchType types.PatchType, data []byte, subresources ...string) (*v3.PipelineSetting, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.PipelineSetting), err
}

func (s *pipelineSettingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *pipelineSettingClient) AddHandler(ctx context.Context, name string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineSettingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineSettingClient) AddLifecycle(ctx context.Context, name string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineSettingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineSettingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineSettingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineSettingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *pipelineSettingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineSettingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineSettingLifecycle) {
	sync := NewPipelineSettingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
