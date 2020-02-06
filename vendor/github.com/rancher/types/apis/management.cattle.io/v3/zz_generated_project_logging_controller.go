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
	ProjectLoggingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectLogging",
	}
	ProjectLoggingResource = metav1.APIResource{
		Name:         "projectloggings",
		SingularName: "projectlogging",
		Namespaced:   true,

		Kind: ProjectLoggingGroupVersionKind.Kind,
	}

	ProjectLoggingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectloggings",
	}
)

func init() {
	resource.Put(ProjectLoggingGroupVersionResource)
}

func NewProjectLogging(namespace, name string, obj ProjectLogging) *ProjectLogging {
	obj.APIVersion, obj.Kind = ProjectLoggingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectLoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectLogging `json:"items"`
}

type ProjectLoggingHandlerFunc func(key string, obj *ProjectLogging) (runtime.Object, error)

type ProjectLoggingChangeHandlerFunc func(obj *ProjectLogging) (runtime.Object, error)

type ProjectLoggingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectLogging, err error)
	Get(namespace, name string) (*ProjectLogging, error)
}

type ProjectLoggingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectLoggingLister
	AddHandler(ctx context.Context, name string, handler ProjectLoggingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectLoggingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectLoggingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectLoggingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectLoggingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectLogging) (*ProjectLogging, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectLogging, error)
	Get(name string, opts metav1.GetOptions) (*ProjectLogging, error)
	Update(*ProjectLogging) (*ProjectLogging, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectLoggingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ProjectLoggingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectLoggingController
	AddHandler(ctx context.Context, name string, sync ProjectLoggingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectLoggingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectLoggingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectLoggingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectLoggingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectLoggingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectLoggingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectLoggingLifecycle)
}

type projectLoggingLister struct {
	controller *projectLoggingController
}

func (l *projectLoggingLister) List(namespace string, selector labels.Selector) (ret []*ProjectLogging, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectLogging))
	})
	return
}

func (l *projectLoggingLister) Get(namespace, name string) (*ProjectLogging, error) {
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
			Group:    ProjectLoggingGroupVersionKind.Group,
			Resource: "projectLogging",
		}, key)
	}
	return obj.(*ProjectLogging), nil
}

type projectLoggingController struct {
	controller.GenericController
}

func (c *projectLoggingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectLoggingController) Lister() ProjectLoggingLister {
	return &projectLoggingLister{
		controller: c,
	}
}

func (c *projectLoggingController) AddHandler(ctx context.Context, name string, handler ProjectLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectLogging); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectLoggingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectLogging); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectLoggingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectLogging); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectLoggingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectLoggingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectLogging); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectLoggingFactory struct {
}

func (c projectLoggingFactory) Object() runtime.Object {
	return &ProjectLogging{}
}

func (c projectLoggingFactory) List() runtime.Object {
	return &ProjectLoggingList{}
}

func (s *projectLoggingClient) Controller() ProjectLoggingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectLoggingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectLoggingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectLoggingController{
		GenericController: genericController,
	}

	s.client.projectLoggingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectLoggingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectLoggingController
}

func (s *projectLoggingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectLoggingClient) Create(o *ProjectLogging) (*ProjectLogging, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) Get(name string, opts metav1.GetOptions) (*ProjectLogging, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectLogging, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) Update(o *ProjectLogging) (*ProjectLogging, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectLoggingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectLoggingClient) List(opts metav1.ListOptions) (*ProjectLoggingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectLoggingList), err
}

func (s *projectLoggingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ProjectLoggingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ProjectLoggingList), err
}

func (s *projectLoggingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectLoggingClient) Patch(o *ProjectLogging, patchType types.PatchType, data []byte, subresources ...string) (*ProjectLogging, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ProjectLogging), err
}

func (s *projectLoggingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectLoggingClient) AddHandler(ctx context.Context, name string, sync ProjectLoggingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectLoggingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectLoggingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectLoggingClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectLoggingLifecycle) {
	sync := NewProjectLoggingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectLoggingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectLoggingLifecycle) {
	sync := NewProjectLoggingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectLoggingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectLoggingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectLoggingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectLoggingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectLoggingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectLoggingLifecycle) {
	sync := NewProjectLoggingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectLoggingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectLoggingLifecycle) {
	sync := NewProjectLoggingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
