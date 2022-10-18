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
	NodeTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NodeTemplate",
	}
	NodeTemplateResource = metav1.APIResource{
		Name:         "nodetemplates",
		SingularName: "nodetemplate",
		Namespaced:   true,

		Kind: NodeTemplateGroupVersionKind.Kind,
	}

	NodeTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "nodetemplates",
	}
)

func init() {
	resource.Put(NodeTemplateGroupVersionResource)
}

// Deprecated: use v3.NodeTemplate instead
type NodeTemplate = v3.NodeTemplate

func NewNodeTemplate(namespace, name string, obj v3.NodeTemplate) *v3.NodeTemplate {
	obj.APIVersion, obj.Kind = NodeTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodeTemplateHandlerFunc func(key string, obj *v3.NodeTemplate) (runtime.Object, error)

type NodeTemplateChangeHandlerFunc func(obj *v3.NodeTemplate) (runtime.Object, error)

type NodeTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.NodeTemplate, err error)
	Get(namespace, name string) (*v3.NodeTemplate, error)
}

type NodeTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NodeTemplateLister
	AddHandler(ctx context.Context, name string, handler NodeTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NodeTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NodeTemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type NodeTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.NodeTemplate) (*v3.NodeTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NodeTemplate, error)
	Get(name string, opts metav1.GetOptions) (*v3.NodeTemplate, error)
	Update(*v3.NodeTemplate) (*v3.NodeTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.NodeTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NodeTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeTemplateController
	AddHandler(ctx context.Context, name string, sync NodeTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NodeTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeTemplateLifecycle)
}

type nodeTemplateLister struct {
	ns         string
	controller *nodeTemplateController
}

func (l *nodeTemplateLister) List(namespace string, selector labels.Selector) (ret []*v3.NodeTemplate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.NodeTemplate))
	})
	return
}

func (l *nodeTemplateLister) Get(namespace, name string) (*v3.NodeTemplate, error) {
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
			Group:    NodeTemplateGroupVersionKind.Group,
			Resource: NodeTemplateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.NodeTemplate), nil
}

type nodeTemplateController struct {
	ns string
	controller.GenericController
}

func (c *nodeTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodeTemplateController) Lister() NodeTemplateLister {
	return &nodeTemplateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *nodeTemplateController) AddHandler(ctx context.Context, name string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.NodeTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodeTemplateFactory struct {
}

func (c nodeTemplateFactory) Object() runtime.Object {
	return &v3.NodeTemplate{}
}

func (c nodeTemplateFactory) List() runtime.Object {
	return &v3.NodeTemplateList{}
}

func (s *nodeTemplateClient) Controller() NodeTemplateController {
	genericController := controller.NewGenericController(s.ns, NodeTemplateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(NodeTemplateGroupVersionResource, NodeTemplateGroupVersionKind.Kind, true))

	return &nodeTemplateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type nodeTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NodeTemplateController
}

func (s *nodeTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *nodeTemplateClient) Create(o *v3.NodeTemplate) (*v3.NodeTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.NodeTemplate), err
}

func (s *nodeTemplateClient) Get(name string, opts metav1.GetOptions) (*v3.NodeTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.NodeTemplate), err
}

func (s *nodeTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.NodeTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.NodeTemplate), err
}

func (s *nodeTemplateClient) Update(o *v3.NodeTemplate) (*v3.NodeTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.NodeTemplate), err
}

func (s *nodeTemplateClient) UpdateStatus(o *v3.NodeTemplate) (*v3.NodeTemplate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.NodeTemplate), err
}

func (s *nodeTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeTemplateClient) List(opts metav1.ListOptions) (*v3.NodeTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.NodeTemplateList), err
}

func (s *nodeTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.NodeTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.NodeTemplateList), err
}

func (s *nodeTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeTemplateClient) Patch(o *v3.NodeTemplate, patchType types.PatchType, data []byte, subresources ...string) (*v3.NodeTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.NodeTemplate), err
}

func (s *nodeTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeTemplateClient) AddHandler(ctx context.Context, name string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *nodeTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
