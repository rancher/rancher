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
	ClusterAlertRuleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterAlertRule",
	}
	ClusterAlertRuleResource = metav1.APIResource{
		Name:         "clusteralertrules",
		SingularName: "clusteralertrule",
		Namespaced:   true,

		Kind: ClusterAlertRuleGroupVersionKind.Kind,
	}

	ClusterAlertRuleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusteralertrules",
	}
)

func init() {
	resource.Put(ClusterAlertRuleGroupVersionResource)
}

// Deprecated: use v3.ClusterAlertRule instead
type ClusterAlertRule = v3.ClusterAlertRule

func NewClusterAlertRule(namespace, name string, obj v3.ClusterAlertRule) *v3.ClusterAlertRule {
	obj.APIVersion, obj.Kind = ClusterAlertRuleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterAlertRuleHandlerFunc func(key string, obj *v3.ClusterAlertRule) (runtime.Object, error)

type ClusterAlertRuleChangeHandlerFunc func(obj *v3.ClusterAlertRule) (runtime.Object, error)

type ClusterAlertRuleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ClusterAlertRule, err error)
	Get(namespace, name string) (*v3.ClusterAlertRule, error)
}

type ClusterAlertRuleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterAlertRuleLister
	AddHandler(ctx context.Context, name string, handler ClusterAlertRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertRuleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterAlertRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterAlertRuleHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterAlertRuleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ClusterAlertRule) (*v3.ClusterAlertRule, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterAlertRule, error)
	Get(name string, opts metav1.GetOptions) (*v3.ClusterAlertRule, error)
	Update(*v3.ClusterAlertRule) (*v3.ClusterAlertRule, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ClusterAlertRuleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterAlertRuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterAlertRuleController
	AddHandler(ctx context.Context, name string, sync ClusterAlertRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertRuleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterAlertRuleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAlertRuleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAlertRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAlertRuleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAlertRuleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAlertRuleLifecycle)
}

type clusterAlertRuleLister struct {
	ns         string
	controller *clusterAlertRuleController
}

func (l *clusterAlertRuleLister) List(namespace string, selector labels.Selector) (ret []*v3.ClusterAlertRule, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ClusterAlertRule))
	})
	return
}

func (l *clusterAlertRuleLister) Get(namespace, name string) (*v3.ClusterAlertRule, error) {
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
			Group:    ClusterAlertRuleGroupVersionKind.Group,
			Resource: ClusterAlertRuleGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ClusterAlertRule), nil
}

type clusterAlertRuleController struct {
	ns string
	controller.GenericController
}

func (c *clusterAlertRuleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterAlertRuleController) Lister() ClusterAlertRuleLister {
	return &clusterAlertRuleLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterAlertRuleController) AddHandler(ctx context.Context, name string, handler ClusterAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertRuleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertRuleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertRuleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterAlertRuleFactory struct {
}

func (c clusterAlertRuleFactory) Object() runtime.Object {
	return &v3.ClusterAlertRule{}
}

func (c clusterAlertRuleFactory) List() runtime.Object {
	return &v3.ClusterAlertRuleList{}
}

func (s *clusterAlertRuleClient) Controller() ClusterAlertRuleController {
	genericController := controller.NewGenericController(s.ns, ClusterAlertRuleGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterAlertRuleGroupVersionResource, ClusterAlertRuleGroupVersionKind.Kind, true))

	return &clusterAlertRuleController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterAlertRuleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterAlertRuleController
}

func (s *clusterAlertRuleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterAlertRuleClient) Create(o *v3.ClusterAlertRule) (*v3.ClusterAlertRule, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ClusterAlertRule), err
}

func (s *clusterAlertRuleClient) Get(name string, opts metav1.GetOptions) (*v3.ClusterAlertRule, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ClusterAlertRule), err
}

func (s *clusterAlertRuleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterAlertRule, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ClusterAlertRule), err
}

func (s *clusterAlertRuleClient) Update(o *v3.ClusterAlertRule) (*v3.ClusterAlertRule, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ClusterAlertRule), err
}

func (s *clusterAlertRuleClient) UpdateStatus(o *v3.ClusterAlertRule) (*v3.ClusterAlertRule, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ClusterAlertRule), err
}

func (s *clusterAlertRuleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterAlertRuleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterAlertRuleClient) List(opts metav1.ListOptions) (*v3.ClusterAlertRuleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ClusterAlertRuleList), err
}

func (s *clusterAlertRuleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterAlertRuleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ClusterAlertRuleList), err
}

func (s *clusterAlertRuleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAlertRuleClient) Patch(o *v3.ClusterAlertRule, patchType types.PatchType, data []byte, subresources ...string) (*v3.ClusterAlertRule, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ClusterAlertRule), err
}

func (s *clusterAlertRuleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterAlertRuleClient) AddHandler(ctx context.Context, name string, sync ClusterAlertRuleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAlertRuleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertRuleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAlertRuleClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterAlertRuleLifecycle) {
	sync := NewClusterAlertRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAlertRuleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAlertRuleLifecycle) {
	sync := NewClusterAlertRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAlertRuleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAlertRuleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAlertRuleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAlertRuleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterAlertRuleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAlertRuleLifecycle) {
	sync := NewClusterAlertRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAlertRuleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAlertRuleLifecycle) {
	sync := NewClusterAlertRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
