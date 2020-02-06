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
	ClusterTemplateRevisionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterTemplateRevision",
	}
	ClusterTemplateRevisionResource = metav1.APIResource{
		Name:         "clustertemplaterevisions",
		SingularName: "clustertemplaterevision",
		Namespaced:   true,

		Kind: ClusterTemplateRevisionGroupVersionKind.Kind,
	}

	ClusterTemplateRevisionGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clustertemplaterevisions",
	}
)

func init() {
	resource.Put(ClusterTemplateRevisionGroupVersionResource)
}

func NewClusterTemplateRevision(namespace, name string, obj ClusterTemplateRevision) *ClusterTemplateRevision {
	obj.APIVersion, obj.Kind = ClusterTemplateRevisionGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterTemplateRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTemplateRevision `json:"items"`
}

type ClusterTemplateRevisionHandlerFunc func(key string, obj *ClusterTemplateRevision) (runtime.Object, error)

type ClusterTemplateRevisionChangeHandlerFunc func(obj *ClusterTemplateRevision) (runtime.Object, error)

type ClusterTemplateRevisionLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterTemplateRevision, err error)
	Get(namespace, name string) (*ClusterTemplateRevision, error)
}

type ClusterTemplateRevisionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterTemplateRevisionLister
	AddHandler(ctx context.Context, name string, handler ClusterTemplateRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateRevisionHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterTemplateRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterTemplateRevisionHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterTemplateRevisionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterTemplateRevision) (*ClusterTemplateRevision, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterTemplateRevision, error)
	Get(name string, opts metav1.GetOptions) (*ClusterTemplateRevision, error)
	Update(*ClusterTemplateRevision) (*ClusterTemplateRevision, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterTemplateRevisionList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterTemplateRevisionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterTemplateRevisionController
	AddHandler(ctx context.Context, name string, sync ClusterTemplateRevisionHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateRevisionHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterTemplateRevisionLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterTemplateRevisionLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterTemplateRevisionHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterTemplateRevisionHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterTemplateRevisionLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterTemplateRevisionLifecycle)
}

type clusterTemplateRevisionLister struct {
	controller *clusterTemplateRevisionController
}

func (l *clusterTemplateRevisionLister) List(namespace string, selector labels.Selector) (ret []*ClusterTemplateRevision, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterTemplateRevision))
	})
	return
}

func (l *clusterTemplateRevisionLister) Get(namespace, name string) (*ClusterTemplateRevision, error) {
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
			Group:    ClusterTemplateRevisionGroupVersionKind.Group,
			Resource: "clusterTemplateRevision",
		}, key)
	}
	return obj.(*ClusterTemplateRevision), nil
}

type clusterTemplateRevisionController struct {
	controller.GenericController
}

func (c *clusterTemplateRevisionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterTemplateRevisionController) Lister() ClusterTemplateRevisionLister {
	return &clusterTemplateRevisionLister{
		controller: c,
	}
}

func (c *clusterTemplateRevisionController) AddHandler(ctx context.Context, name string, handler ClusterTemplateRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplateRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateRevisionController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterTemplateRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplateRevision); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateRevisionController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterTemplateRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplateRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateRevisionController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterTemplateRevisionHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplateRevision); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterTemplateRevisionFactory struct {
}

func (c clusterTemplateRevisionFactory) Object() runtime.Object {
	return &ClusterTemplateRevision{}
}

func (c clusterTemplateRevisionFactory) List() runtime.Object {
	return &ClusterTemplateRevisionList{}
}

func (s *clusterTemplateRevisionClient) Controller() ClusterTemplateRevisionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterTemplateRevisionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterTemplateRevisionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterTemplateRevisionController{
		GenericController: genericController,
	}

	s.client.clusterTemplateRevisionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterTemplateRevisionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterTemplateRevisionController
}

func (s *clusterTemplateRevisionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterTemplateRevisionClient) Create(o *ClusterTemplateRevision) (*ClusterTemplateRevision, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterTemplateRevision), err
}

func (s *clusterTemplateRevisionClient) Get(name string, opts metav1.GetOptions) (*ClusterTemplateRevision, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterTemplateRevision), err
}

func (s *clusterTemplateRevisionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterTemplateRevision, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterTemplateRevision), err
}

func (s *clusterTemplateRevisionClient) Update(o *ClusterTemplateRevision) (*ClusterTemplateRevision, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterTemplateRevision), err
}

func (s *clusterTemplateRevisionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterTemplateRevisionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterTemplateRevisionClient) List(opts metav1.ListOptions) (*ClusterTemplateRevisionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterTemplateRevisionList), err
}

func (s *clusterTemplateRevisionClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterTemplateRevisionList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterTemplateRevisionList), err
}

func (s *clusterTemplateRevisionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterTemplateRevisionClient) Patch(o *ClusterTemplateRevision, patchType types.PatchType, data []byte, subresources ...string) (*ClusterTemplateRevision, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterTemplateRevision), err
}

func (s *clusterTemplateRevisionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterTemplateRevisionClient) AddHandler(ctx context.Context, name string, sync ClusterTemplateRevisionHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterTemplateRevisionClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateRevisionHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterTemplateRevisionClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterTemplateRevisionLifecycle) {
	sync := NewClusterTemplateRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterTemplateRevisionClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterTemplateRevisionLifecycle) {
	sync := NewClusterTemplateRevisionLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterTemplateRevisionClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterTemplateRevisionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterTemplateRevisionClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterTemplateRevisionHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterTemplateRevisionClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterTemplateRevisionLifecycle) {
	sync := NewClusterTemplateRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterTemplateRevisionClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterTemplateRevisionLifecycle) {
	sync := NewClusterTemplateRevisionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
