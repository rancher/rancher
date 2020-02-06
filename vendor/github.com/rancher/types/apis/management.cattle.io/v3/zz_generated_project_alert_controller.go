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
	ProjectAlertGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectAlert",
	}
	ProjectAlertResource = metav1.APIResource{
		Name:         "projectalerts",
		SingularName: "projectalert",
		Namespaced:   true,

		Kind: ProjectAlertGroupVersionKind.Kind,
	}

	ProjectAlertGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectalerts",
	}
)

func init() {
	resource.Put(ProjectAlertGroupVersionResource)
}

func NewProjectAlert(namespace, name string, obj ProjectAlert) *ProjectAlert {
	obj.APIVersion, obj.Kind = ProjectAlertGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectAlert `json:"items"`
}

type ProjectAlertHandlerFunc func(key string, obj *ProjectAlert) (runtime.Object, error)

type ProjectAlertChangeHandlerFunc func(obj *ProjectAlert) (runtime.Object, error)

type ProjectAlertLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectAlert, err error)
	Get(namespace, name string) (*ProjectAlert, error)
}

type ProjectAlertController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectAlertLister
	AddHandler(ctx context.Context, name string, handler ProjectAlertHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectAlertHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectAlertHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectAlertInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectAlert) (*ProjectAlert, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlert, error)
	Get(name string, opts metav1.GetOptions) (*ProjectAlert, error)
	Update(*ProjectAlert) (*ProjectAlert, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectAlertList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ProjectAlertList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectAlertController
	AddHandler(ctx context.Context, name string, sync ProjectAlertHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertLifecycle)
}

type projectAlertLister struct {
	controller *projectAlertController
}

func (l *projectAlertLister) List(namespace string, selector labels.Selector) (ret []*ProjectAlert, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectAlert))
	})
	return
}

func (l *projectAlertLister) Get(namespace, name string) (*ProjectAlert, error) {
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
			Group:    ProjectAlertGroupVersionKind.Group,
			Resource: "projectAlert",
		}, key)
	}
	return obj.(*ProjectAlert), nil
}

type projectAlertController struct {
	controller.GenericController
}

func (c *projectAlertController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectAlertController) Lister() ProjectAlertLister {
	return &projectAlertLister{
		controller: c,
	}
}

func (c *projectAlertController) AddHandler(ctx context.Context, name string, handler ProjectAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlert); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlert); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlert); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectAlertHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlert); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectAlertFactory struct {
}

func (c projectAlertFactory) Object() runtime.Object {
	return &ProjectAlert{}
}

func (c projectAlertFactory) List() runtime.Object {
	return &ProjectAlertList{}
}

func (s *projectAlertClient) Controller() ProjectAlertController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectAlertControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectAlertGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectAlertController{
		GenericController: genericController,
	}

	s.client.projectAlertControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectAlertClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectAlertController
}

func (s *projectAlertClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectAlertClient) Create(o *ProjectAlert) (*ProjectAlert, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) Get(name string, opts metav1.GetOptions) (*ProjectAlert, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlert, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) Update(o *ProjectAlert) (*ProjectAlert, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectAlertClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectAlertClient) List(opts metav1.ListOptions) (*ProjectAlertList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectAlertList), err
}

func (s *projectAlertClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ProjectAlertList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ProjectAlertList), err
}

func (s *projectAlertClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectAlertClient) Patch(o *ProjectAlert, patchType types.PatchType, data []byte, subresources ...string) (*ProjectAlert, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ProjectAlert), err
}

func (s *projectAlertClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectAlertClient) AddHandler(ctx context.Context, name string, sync ProjectAlertHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertLifecycle) {
	sync := NewProjectAlertLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertLifecycle) {
	sync := NewProjectAlertLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectAlertClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertLifecycle) {
	sync := NewProjectAlertLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertLifecycle) {
	sync := NewProjectAlertLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
