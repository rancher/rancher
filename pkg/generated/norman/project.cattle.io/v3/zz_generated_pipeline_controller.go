package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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
	PipelineGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Pipeline",
	}
	PipelineResource = metav1.APIResource{
		Name:         "pipelines",
		SingularName: "pipeline",
		Namespaced:   true,

		Kind: PipelineGroupVersionKind.Kind,
	}

	PipelineGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "pipelines",
	}
)

func init() {
	resource.Put(PipelineGroupVersionResource)
}

// Deprecated use v3.Pipeline instead
type Pipeline = v3.Pipeline

func NewPipeline(namespace, name string, obj v3.Pipeline) *v3.Pipeline {
	obj.APIVersion, obj.Kind = PipelineGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PipelineHandlerFunc func(key string, obj *v3.Pipeline) (runtime.Object, error)

type PipelineChangeHandlerFunc func(obj *v3.Pipeline) (runtime.Object, error)

type PipelineLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Pipeline, err error)
	Get(namespace, name string) (*v3.Pipeline, error)
}

type PipelineController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PipelineLister
	AddHandler(ctx context.Context, name string, handler PipelineHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PipelineHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PipelineHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PipelineInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Pipeline) (*v3.Pipeline, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Pipeline, error)
	Get(name string, opts metav1.GetOptions) (*v3.Pipeline, error)
	Update(*v3.Pipeline) (*v3.Pipeline, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PipelineList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PipelineList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PipelineController
	AddHandler(ctx context.Context, name string, sync PipelineHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PipelineLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineLifecycle)
}

type pipelineLister struct {
	ns         string
	controller *pipelineController
}

func (l *pipelineLister) List(namespace string, selector labels.Selector) (ret []*v3.Pipeline, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Pipeline))
	})
	return
}

func (l *pipelineLister) Get(namespace, name string) (*v3.Pipeline, error) {
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
			Group:    PipelineGroupVersionKind.Group,
			Resource: PipelineGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Pipeline), nil
}

type pipelineController struct {
	ns string
	controller.GenericController
}

func (c *pipelineController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *pipelineController) Lister() PipelineLister {
	return &pipelineLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *pipelineController) AddHandler(ctx context.Context, name string, handler PipelineHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Pipeline); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PipelineHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Pipeline); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PipelineHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Pipeline); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PipelineHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Pipeline); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type pipelineFactory struct {
}

func (c pipelineFactory) Object() runtime.Object {
	return &v3.Pipeline{}
}

func (c pipelineFactory) List() runtime.Object {
	return &v3.PipelineList{}
}

func (s *pipelineClient) Controller() PipelineController {
	genericController := controller.NewGenericController(s.ns, PipelineGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PipelineGroupVersionResource, PipelineGroupVersionKind.Kind, true))

	return &pipelineController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type pipelineClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PipelineController
}

func (s *pipelineClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *pipelineClient) Create(o *v3.Pipeline) (*v3.Pipeline, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Pipeline), err
}

func (s *pipelineClient) Get(name string, opts metav1.GetOptions) (*v3.Pipeline, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Pipeline), err
}

func (s *pipelineClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Pipeline, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Pipeline), err
}

func (s *pipelineClient) Update(o *v3.Pipeline) (*v3.Pipeline, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Pipeline), err
}

func (s *pipelineClient) UpdateStatus(o *v3.Pipeline) (*v3.Pipeline, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Pipeline), err
}

func (s *pipelineClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineClient) List(opts metav1.ListOptions) (*v3.PipelineList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PipelineList), err
}

func (s *pipelineClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PipelineList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PipelineList), err
}

func (s *pipelineClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineClient) Patch(o *v3.Pipeline, patchType types.PatchType, data []byte, subresources ...string) (*v3.Pipeline, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Pipeline), err
}

func (s *pipelineClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *pipelineClient) AddHandler(ctx context.Context, name string, sync PipelineHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineClient) AddLifecycle(ctx context.Context, name string, lifecycle PipelineLifecycle) {
	sync := NewPipelineLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineLifecycle) {
	sync := NewPipelineLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *pipelineClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineLifecycle) {
	sync := NewPipelineLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineLifecycle) {
	sync := NewPipelineLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
