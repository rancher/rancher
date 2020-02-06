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
	PipelineExecutionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PipelineExecution",
	}
	PipelineExecutionResource = metav1.APIResource{
		Name:         "pipelineexecutions",
		SingularName: "pipelineexecution",
		Namespaced:   true,

		Kind: PipelineExecutionGroupVersionKind.Kind,
	}

	PipelineExecutionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "pipelineexecutions",
	}
)

func init() {
	resource.Put(PipelineExecutionGroupVersionResource)
}

func NewPipelineExecution(namespace, name string, obj PipelineExecution) *PipelineExecution {
	obj.APIVersion, obj.Kind = PipelineExecutionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PipelineExecutionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineExecution `json:"items"`
}

type PipelineExecutionHandlerFunc func(key string, obj *PipelineExecution) (runtime.Object, error)

type PipelineExecutionChangeHandlerFunc func(obj *PipelineExecution) (runtime.Object, error)

type PipelineExecutionLister interface {
	List(namespace string, selector labels.Selector) (ret []*PipelineExecution, err error)
	Get(namespace, name string) (*PipelineExecution, error)
}

type PipelineExecutionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PipelineExecutionLister
	AddHandler(ctx context.Context, name string, handler PipelineExecutionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineExecutionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PipelineExecutionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PipelineExecutionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PipelineExecutionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*PipelineExecution) (*PipelineExecution, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineExecution, error)
	Get(name string, opts metav1.GetOptions) (*PipelineExecution, error)
	Update(*PipelineExecution) (*PipelineExecution, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PipelineExecutionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*PipelineExecutionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PipelineExecutionController
	AddHandler(ctx context.Context, name string, sync PipelineExecutionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineExecutionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PipelineExecutionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineExecutionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineExecutionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineExecutionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineExecutionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineExecutionLifecycle)
}

type pipelineExecutionLister struct {
	controller *pipelineExecutionController
}

func (l *pipelineExecutionLister) List(namespace string, selector labels.Selector) (ret []*PipelineExecution, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PipelineExecution))
	})
	return
}

func (l *pipelineExecutionLister) Get(namespace, name string) (*PipelineExecution, error) {
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
			Group:    PipelineExecutionGroupVersionKind.Group,
			Resource: "pipelineExecution",
		}, key)
	}
	return obj.(*PipelineExecution), nil
}

type pipelineExecutionController struct {
	controller.GenericController
}

func (c *pipelineExecutionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *pipelineExecutionController) Lister() PipelineExecutionLister {
	return &pipelineExecutionLister{
		controller: c,
	}
}

func (c *pipelineExecutionController) AddHandler(ctx context.Context, name string, handler PipelineExecutionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineExecution); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineExecutionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PipelineExecutionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineExecution); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineExecutionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PipelineExecutionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineExecution); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *pipelineExecutionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PipelineExecutionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PipelineExecution); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type pipelineExecutionFactory struct {
}

func (c pipelineExecutionFactory) Object() runtime.Object {
	return &PipelineExecution{}
}

func (c pipelineExecutionFactory) List() runtime.Object {
	return &PipelineExecutionList{}
}

func (s *pipelineExecutionClient) Controller() PipelineExecutionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.pipelineExecutionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PipelineExecutionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &pipelineExecutionController{
		GenericController: genericController,
	}

	s.client.pipelineExecutionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type pipelineExecutionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PipelineExecutionController
}

func (s *pipelineExecutionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *pipelineExecutionClient) Create(o *PipelineExecution) (*PipelineExecution, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) Get(name string, opts metav1.GetOptions) (*PipelineExecution, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineExecution, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) Update(o *PipelineExecution) (*PipelineExecution, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineExecutionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineExecutionClient) List(opts metav1.ListOptions) (*PipelineExecutionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PipelineExecutionList), err
}

func (s *pipelineExecutionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*PipelineExecutionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*PipelineExecutionList), err
}

func (s *pipelineExecutionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineExecutionClient) Patch(o *PipelineExecution, patchType types.PatchType, data []byte, subresources ...string) (*PipelineExecution, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *pipelineExecutionClient) AddHandler(ctx context.Context, name string, sync PipelineExecutionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineExecutionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PipelineExecutionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineExecutionClient) AddLifecycle(ctx context.Context, name string, lifecycle PipelineExecutionLifecycle) {
	sync := NewPipelineExecutionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *pipelineExecutionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PipelineExecutionLifecycle) {
	sync := NewPipelineExecutionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *pipelineExecutionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PipelineExecutionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineExecutionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PipelineExecutionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *pipelineExecutionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PipelineExecutionLifecycle) {
	sync := NewPipelineExecutionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *pipelineExecutionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PipelineExecutionLifecycle) {
	sync := NewPipelineExecutionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
