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

func NewPipeline(namespace, name string, obj Pipeline) *Pipeline {
	obj.APIVersion, obj.Kind = PipelineGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}

type PipelineHandlerFunc func(key string, obj *Pipeline) (runtime.Object, error)

type PipelineChangeHandlerFunc func(obj *Pipeline) (runtime.Object, error)

type PipelineLister interface {
	List(namespace string, selector labels.Selector) (ret []*Pipeline, err error)
	Get(namespace, name string) (*Pipeline, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PipelineInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Pipeline) (*Pipeline, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Pipeline, error)
	Get(name string, opts metav1.GetOptions) (*Pipeline, error)
	Update(*Pipeline) (*Pipeline, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PipelineList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*PipelineList, error)
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
	controller *pipelineController
}

func (l *pipelineLister) List(namespace string, selector labels.Selector) (ret []*Pipeline, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Pipeline))
	})
	return
}

func (l *pipelineLister) Get(namespace, name string) (*Pipeline, error) {
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
			Resource: "pipeline",
		}, key)
	}
	return obj.(*Pipeline), nil
}

type pipelineController struct {
	controller.GenericController
}

func (c *pipelineController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *pipelineController) Lister() PipelineLister {
	return &pipelineLister{
		controller: c,
	}
}

func (c *pipelineController) AddHandler(ctx context.Context, name string, handler PipelineHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Pipeline); ok {
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
		} else if v, ok := obj.(*Pipeline); ok {
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
		} else if v, ok := obj.(*Pipeline); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*Pipeline); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type pipelineFactory struct {
}

func (c pipelineFactory) Object() runtime.Object {
	return &Pipeline{}
}

func (c pipelineFactory) List() runtime.Object {
	return &PipelineList{}
}

func (s *pipelineClient) Controller() PipelineController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.pipelineControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PipelineGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &pipelineController{
		GenericController: genericController,
	}

	s.client.pipelineControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *pipelineClient) Create(o *Pipeline) (*Pipeline, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Pipeline), err
}

func (s *pipelineClient) Get(name string, opts metav1.GetOptions) (*Pipeline, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Pipeline), err
}

func (s *pipelineClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Pipeline, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Pipeline), err
}

func (s *pipelineClient) Update(o *Pipeline) (*Pipeline, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Pipeline), err
}

func (s *pipelineClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineClient) List(opts metav1.ListOptions) (*PipelineList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PipelineList), err
}

func (s *pipelineClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*PipelineList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*PipelineList), err
}

func (s *pipelineClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineClient) Patch(o *Pipeline, patchType types.PatchType, data []byte, subresources ...string) (*Pipeline, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Pipeline), err
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
