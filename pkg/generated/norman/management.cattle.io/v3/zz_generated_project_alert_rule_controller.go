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
	ProjectAlertRuleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectAlertRule",
	}
	ProjectAlertRuleResource = metav1.APIResource{
		Name:         "projectalertrules",
		SingularName: "projectalertrule",
		Namespaced:   true,

		Kind: ProjectAlertRuleGroupVersionKind.Kind,
	}

	ProjectAlertRuleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectalertrules",
	}
)

func init() {
	resource.Put(ProjectAlertRuleGroupVersionResource)
}

// Deprecated: use v3.ProjectAlertRule instead
type ProjectAlertRule = v3.ProjectAlertRule

func NewProjectAlertRule(namespace, name string, obj v3.ProjectAlertRule) *v3.ProjectAlertRule {
	obj.APIVersion, obj.Kind = ProjectAlertRuleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectAlertRuleHandlerFunc func(key string, obj *v3.ProjectAlertRule) (runtime.Object, error)

type ProjectAlertRuleChangeHandlerFunc func(obj *v3.ProjectAlertRule) (runtime.Object, error)

type ProjectAlertRuleLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ProjectAlertRule, err error)
	Get(namespace, name string) (*v3.ProjectAlertRule, error)
}

type ProjectAlertRuleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectAlertRuleLister
	AddHandler(ctx context.Context, name string, handler ProjectAlertRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertRuleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectAlertRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectAlertRuleHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ProjectAlertRuleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ProjectAlertRule) (*v3.ProjectAlertRule, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ProjectAlertRule, error)
	Get(name string, opts metav1.GetOptions) (*v3.ProjectAlertRule, error)
	Update(*v3.ProjectAlertRule) (*v3.ProjectAlertRule, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ProjectAlertRuleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectAlertRuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectAlertRuleController
	AddHandler(ctx context.Context, name string, sync ProjectAlertRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertRuleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertRuleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertRuleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertRuleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertRuleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertRuleLifecycle)
}

type projectAlertRuleLister struct {
	ns         string
	controller *projectAlertRuleController
}

func (l *projectAlertRuleLister) List(namespace string, selector labels.Selector) (ret []*v3.ProjectAlertRule, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ProjectAlertRule))
	})
	return
}

func (l *projectAlertRuleLister) Get(namespace, name string) (*v3.ProjectAlertRule, error) {
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
			Group:    ProjectAlertRuleGroupVersionKind.Group,
			Resource: ProjectAlertRuleGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ProjectAlertRule), nil
}

type projectAlertRuleController struct {
	ns string
	controller.GenericController
}

func (c *projectAlertRuleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectAlertRuleController) Lister() ProjectAlertRuleLister {
	return &projectAlertRuleLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *projectAlertRuleController) AddHandler(ctx context.Context, name string, handler ProjectAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectAlertRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertRuleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectAlertRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertRuleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectAlertRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertRuleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ProjectAlertRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectAlertRuleFactory struct {
}

func (c projectAlertRuleFactory) Object() runtime.Object {
	return &v3.ProjectAlertRule{}
}

func (c projectAlertRuleFactory) List() runtime.Object {
	return &v3.ProjectAlertRuleList{}
}

func (s *projectAlertRuleClient) Controller() ProjectAlertRuleController {
	genericController := controller.NewGenericController(s.ns, ProjectAlertRuleGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ProjectAlertRuleGroupVersionResource, ProjectAlertRuleGroupVersionKind.Kind, true))

	return &projectAlertRuleController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type projectAlertRuleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectAlertRuleController
}

func (s *projectAlertRuleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectAlertRuleClient) Create(o *v3.ProjectAlertRule) (*v3.ProjectAlertRule, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ProjectAlertRule), err
}

func (s *projectAlertRuleClient) Get(name string, opts metav1.GetOptions) (*v3.ProjectAlertRule, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ProjectAlertRule), err
}

func (s *projectAlertRuleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ProjectAlertRule, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ProjectAlertRule), err
}

func (s *projectAlertRuleClient) Update(o *v3.ProjectAlertRule) (*v3.ProjectAlertRule, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ProjectAlertRule), err
}

func (s *projectAlertRuleClient) UpdateStatus(o *v3.ProjectAlertRule) (*v3.ProjectAlertRule, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ProjectAlertRule), err
}

func (s *projectAlertRuleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectAlertRuleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectAlertRuleClient) List(opts metav1.ListOptions) (*v3.ProjectAlertRuleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ProjectAlertRuleList), err
}

func (s *projectAlertRuleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ProjectAlertRuleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ProjectAlertRuleList), err
}

func (s *projectAlertRuleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectAlertRuleClient) Patch(o *v3.ProjectAlertRule, patchType types.PatchType, data []byte, subresources ...string) (*v3.ProjectAlertRule, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ProjectAlertRule), err
}

func (s *projectAlertRuleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectAlertRuleClient) AddHandler(ctx context.Context, name string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertRuleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertRuleClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertRuleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
