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

// Deprecated use v3.PipelineExecution instead
type PipelineExecution = v3.PipelineExecution

func NewPipelineExecution(namespace, name string, obj v3.PipelineExecution) *v3.PipelineExecution {
	obj.APIVersion, obj.Kind = PipelineExecutionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PipelineExecutionHandlerFunc func(key string, obj *v3.PipelineExecution) (runtime.Object, error)

type PipelineExecutionChangeHandlerFunc func(obj *v3.PipelineExecution) (runtime.Object, error)

type PipelineExecutionLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.PipelineExecution, err error)
	Get(namespace, name string) (*v3.PipelineExecution, error)
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
}

type PipelineExecutionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.PipelineExecution) (*v3.PipelineExecution, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PipelineExecution, error)
	Get(name string, opts metav1.GetOptions) (*v3.PipelineExecution, error)
	Update(*v3.PipelineExecution) (*v3.PipelineExecution, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PipelineExecutionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PipelineExecutionList, error)
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
	ns         string
	controller *pipelineExecutionController
}

func (l *pipelineExecutionLister) List(namespace string, selector labels.Selector) (ret []*v3.PipelineExecution, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.PipelineExecution))
	})
	return
}

func (l *pipelineExecutionLister) Get(namespace, name string) (*v3.PipelineExecution, error) {
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
			Resource: PipelineExecutionGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.PipelineExecution), nil
}

type pipelineExecutionController struct {
	ns string
	controller.GenericController
}

func (c *pipelineExecutionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *pipelineExecutionController) Lister() PipelineExecutionLister {
	return &pipelineExecutionLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *pipelineExecutionController) AddHandler(ctx context.Context, name string, handler PipelineExecutionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PipelineExecution); ok {
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
		} else if v, ok := obj.(*v3.PipelineExecution); ok {
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
		} else if v, ok := obj.(*v3.PipelineExecution); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*v3.PipelineExecution); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type pipelineExecutionFactory struct {
}

func (c pipelineExecutionFactory) Object() runtime.Object {
	return &v3.PipelineExecution{}
}

func (c pipelineExecutionFactory) List() runtime.Object {
	return &v3.PipelineExecutionList{}
}

func (s *pipelineExecutionClient) Controller() PipelineExecutionController {
	genericController := controller.NewGenericController(s.ns, PipelineExecutionGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PipelineExecutionGroupVersionResource, PipelineExecutionGroupVersionKind.Kind, true))

	return &pipelineExecutionController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *pipelineExecutionClient) Create(o *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.PipelineExecution), err
}

func (s *pipelineExecutionClient) Get(name string, opts metav1.GetOptions) (*v3.PipelineExecution, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.PipelineExecution), err
}

func (s *pipelineExecutionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PipelineExecution, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.PipelineExecution), err
}

func (s *pipelineExecutionClient) Update(o *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.PipelineExecution), err
}

func (s *pipelineExecutionClient) UpdateStatus(o *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.PipelineExecution), err
}

func (s *pipelineExecutionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineExecutionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineExecutionClient) List(opts metav1.ListOptions) (*v3.PipelineExecutionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PipelineExecutionList), err
}

func (s *pipelineExecutionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PipelineExecutionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PipelineExecutionList), err
}

func (s *pipelineExecutionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineExecutionClient) Patch(o *v3.PipelineExecution, patchType types.PatchType, data []byte, subresources ...string) (*v3.PipelineExecution, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.PipelineExecution), err
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
