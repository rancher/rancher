package v1alpha3

import (
	"context"
	"time"

	"github.com/knative/pkg/apis/istio/v1alpha3"
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
	DestinationRuleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DestinationRule",
	}
	DestinationRuleResource = metav1.APIResource{
		Name:         "destinationrules",
		SingularName: "destinationrule",
		Namespaced:   true,

		Kind: DestinationRuleGroupVersionKind.Kind,
	}

	DestinationRuleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "destinationrules",
	}
)

func init() {
	resource.Put(DestinationRuleGroupVersionResource)
}

func NewDestinationRule(namespace, name string, obj v1alpha3.DestinationRule) *v1alpha3.DestinationRule {
	obj.APIVersion, obj.Kind = DestinationRuleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DestinationRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1alpha3.DestinationRule `json:"items"`
}

type DestinationRuleHandlerFunc func(key string, obj *v1alpha3.DestinationRule) (runtime.Object, error)

type DestinationRuleChangeHandlerFunc func(obj *v1alpha3.DestinationRule) (runtime.Object, error)

type DestinationRuleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1alpha3.DestinationRule, err error)
	Get(namespace, name string) (*v1alpha3.DestinationRule, error)
}

type DestinationRuleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DestinationRuleLister
	AddHandler(ctx context.Context, name string, handler DestinationRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DestinationRuleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DestinationRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler DestinationRuleHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DestinationRuleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1alpha3.DestinationRule) (*v1alpha3.DestinationRule, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1alpha3.DestinationRule, error)
	Get(name string, opts metav1.GetOptions) (*v1alpha3.DestinationRule, error)
	Update(*v1alpha3.DestinationRule) (*v1alpha3.DestinationRule, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DestinationRuleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*DestinationRuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DestinationRuleController
	AddHandler(ctx context.Context, name string, sync DestinationRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DestinationRuleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DestinationRuleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DestinationRuleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DestinationRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DestinationRuleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DestinationRuleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DestinationRuleLifecycle)
}

type destinationRuleLister struct {
	controller *destinationRuleController
}

func (l *destinationRuleLister) List(namespace string, selector labels.Selector) (ret []*v1alpha3.DestinationRule, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1alpha3.DestinationRule))
	})
	return
}

func (l *destinationRuleLister) Get(namespace, name string) (*v1alpha3.DestinationRule, error) {
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
			Group:    DestinationRuleGroupVersionKind.Group,
			Resource: "destinationRule",
		}, key)
	}
	return obj.(*v1alpha3.DestinationRule), nil
}

type destinationRuleController struct {
	controller.GenericController
}

func (c *destinationRuleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *destinationRuleController) Lister() DestinationRuleLister {
	return &destinationRuleLister{
		controller: c,
	}
}

func (c *destinationRuleController) AddHandler(ctx context.Context, name string, handler DestinationRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.DestinationRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *destinationRuleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler DestinationRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.DestinationRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *destinationRuleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DestinationRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.DestinationRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *destinationRuleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler DestinationRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1alpha3.DestinationRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type destinationRuleFactory struct {
}

func (c destinationRuleFactory) Object() runtime.Object {
	return &v1alpha3.DestinationRule{}
}

func (c destinationRuleFactory) List() runtime.Object {
	return &DestinationRuleList{}
}

func (s *destinationRuleClient) Controller() DestinationRuleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.destinationRuleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DestinationRuleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &destinationRuleController{
		GenericController: genericController,
	}

	s.client.destinationRuleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type destinationRuleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DestinationRuleController
}

func (s *destinationRuleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *destinationRuleClient) Create(o *v1alpha3.DestinationRule) (*v1alpha3.DestinationRule, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1alpha3.DestinationRule), err
}

func (s *destinationRuleClient) Get(name string, opts metav1.GetOptions) (*v1alpha3.DestinationRule, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1alpha3.DestinationRule), err
}

func (s *destinationRuleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1alpha3.DestinationRule, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1alpha3.DestinationRule), err
}

func (s *destinationRuleClient) Update(o *v1alpha3.DestinationRule) (*v1alpha3.DestinationRule, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1alpha3.DestinationRule), err
}

func (s *destinationRuleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *destinationRuleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *destinationRuleClient) List(opts metav1.ListOptions) (*DestinationRuleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DestinationRuleList), err
}

func (s *destinationRuleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*DestinationRuleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*DestinationRuleList), err
}

func (s *destinationRuleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *destinationRuleClient) Patch(o *v1alpha3.DestinationRule, patchType types.PatchType, data []byte, subresources ...string) (*v1alpha3.DestinationRule, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1alpha3.DestinationRule), err
}

func (s *destinationRuleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *destinationRuleClient) AddHandler(ctx context.Context, name string, sync DestinationRuleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *destinationRuleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DestinationRuleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *destinationRuleClient) AddLifecycle(ctx context.Context, name string, lifecycle DestinationRuleLifecycle) {
	sync := NewDestinationRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *destinationRuleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DestinationRuleLifecycle) {
	sync := NewDestinationRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *destinationRuleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DestinationRuleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *destinationRuleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DestinationRuleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *destinationRuleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DestinationRuleLifecycle) {
	sync := NewDestinationRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *destinationRuleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DestinationRuleLifecycle) {
	sync := NewDestinationRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
