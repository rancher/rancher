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
	ProjectNetworkPolicyGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectNetworkPolicy",
	}
	ProjectNetworkPolicyResource = metav1.APIResource{
		Name:         "projectnetworkpolicies",
		SingularName: "projectnetworkpolicy",
		Namespaced:   true,

		Kind: ProjectNetworkPolicyGroupVersionKind.Kind,
	}

	ProjectNetworkPolicyGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectnetworkpolicies",
	}
)

func init() {
	resource.Put(ProjectNetworkPolicyGroupVersionResource)
}

func NewProjectNetworkPolicy(namespace, name string, obj ProjectNetworkPolicy) *ProjectNetworkPolicy {
	obj.APIVersion, obj.Kind = ProjectNetworkPolicyGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectNetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectNetworkPolicy `json:"items"`
}

type ProjectNetworkPolicyHandlerFunc func(key string, obj *ProjectNetworkPolicy) (runtime.Object, error)

type ProjectNetworkPolicyChangeHandlerFunc func(obj *ProjectNetworkPolicy) (runtime.Object, error)

type ProjectNetworkPolicyLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectNetworkPolicy, err error)
	Get(namespace, name string) (*ProjectNetworkPolicy, error)
}

type ProjectNetworkPolicyController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectNetworkPolicyLister
	AddHandler(ctx context.Context, name string, handler ProjectNetworkPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectNetworkPolicyHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectNetworkPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectNetworkPolicyHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectNetworkPolicyInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error)
	Get(name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error)
	Update(*ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectNetworkPolicyList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ProjectNetworkPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectNetworkPolicyController
	AddHandler(ctx context.Context, name string, sync ProjectNetworkPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectNetworkPolicyHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectNetworkPolicyLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectNetworkPolicyLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectNetworkPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectNetworkPolicyHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectNetworkPolicyLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectNetworkPolicyLifecycle)
}

type projectNetworkPolicyLister struct {
	controller *projectNetworkPolicyController
}

func (l *projectNetworkPolicyLister) List(namespace string, selector labels.Selector) (ret []*ProjectNetworkPolicy, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectNetworkPolicy))
	})
	return
}

func (l *projectNetworkPolicyLister) Get(namespace, name string) (*ProjectNetworkPolicy, error) {
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
			Group:    ProjectNetworkPolicyGroupVersionKind.Group,
			Resource: "projectNetworkPolicy",
		}, key)
	}
	return obj.(*ProjectNetworkPolicy), nil
}

type projectNetworkPolicyController struct {
	controller.GenericController
}

func (c *projectNetworkPolicyController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectNetworkPolicyController) Lister() ProjectNetworkPolicyLister {
	return &projectNetworkPolicyLister{
		controller: c,
	}
}

func (c *projectNetworkPolicyController) AddHandler(ctx context.Context, name string, handler ProjectNetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectNetworkPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectNetworkPolicyController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectNetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectNetworkPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectNetworkPolicyController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectNetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectNetworkPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectNetworkPolicyController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectNetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectNetworkPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectNetworkPolicyFactory struct {
}

func (c projectNetworkPolicyFactory) Object() runtime.Object {
	return &ProjectNetworkPolicy{}
}

func (c projectNetworkPolicyFactory) List() runtime.Object {
	return &ProjectNetworkPolicyList{}
}

func (s *projectNetworkPolicyClient) Controller() ProjectNetworkPolicyController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectNetworkPolicyControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectNetworkPolicyGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectNetworkPolicyController{
		GenericController: genericController,
	}

	s.client.projectNetworkPolicyControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectNetworkPolicyClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectNetworkPolicyController
}

func (s *projectNetworkPolicyClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectNetworkPolicyClient) Create(o *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) Get(name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) Update(o *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectNetworkPolicyClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectNetworkPolicyClient) List(opts metav1.ListOptions) (*ProjectNetworkPolicyList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectNetworkPolicyList), err
}

func (s *projectNetworkPolicyClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ProjectNetworkPolicyList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ProjectNetworkPolicyList), err
}

func (s *projectNetworkPolicyClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectNetworkPolicyClient) Patch(o *ProjectNetworkPolicy, patchType types.PatchType, data []byte, subresources ...string) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectNetworkPolicyClient) AddHandler(ctx context.Context, name string, sync ProjectNetworkPolicyHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectNetworkPolicyClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectNetworkPolicyHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectNetworkPolicyClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectNetworkPolicyLifecycle) {
	sync := NewProjectNetworkPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectNetworkPolicyClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectNetworkPolicyLifecycle) {
	sync := NewProjectNetworkPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectNetworkPolicyClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectNetworkPolicyHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectNetworkPolicyClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectNetworkPolicyHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectNetworkPolicyClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectNetworkPolicyLifecycle) {
	sync := NewProjectNetworkPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectNetworkPolicyClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectNetworkPolicyLifecycle) {
	sync := NewProjectNetworkPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
