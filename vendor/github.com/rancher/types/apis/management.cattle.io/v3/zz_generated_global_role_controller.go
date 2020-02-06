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
	GlobalRoleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalRole",
	}
	GlobalRoleResource = metav1.APIResource{
		Name:         "globalroles",
		SingularName: "globalrole",
		Namespaced:   false,
		Kind:         GlobalRoleGroupVersionKind.Kind,
	}

	GlobalRoleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "globalroles",
	}
)

func init() {
	resource.Put(GlobalRoleGroupVersionResource)
}

func NewGlobalRole(namespace, name string, obj GlobalRole) *GlobalRole {
	obj.APIVersion, obj.Kind = GlobalRoleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type GlobalRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalRole `json:"items"`
}

type GlobalRoleHandlerFunc func(key string, obj *GlobalRole) (runtime.Object, error)

type GlobalRoleChangeHandlerFunc func(obj *GlobalRole) (runtime.Object, error)

type GlobalRoleLister interface {
	List(namespace string, selector labels.Selector) (ret []*GlobalRole, err error)
	Get(namespace, name string) (*GlobalRole, error)
}

type GlobalRoleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() GlobalRoleLister
	AddHandler(ctx context.Context, name string, handler GlobalRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler GlobalRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler GlobalRoleHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GlobalRoleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*GlobalRole) (*GlobalRole, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalRole, error)
	Get(name string, opts metav1.GetOptions) (*GlobalRole, error)
	Update(*GlobalRole) (*GlobalRole, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GlobalRoleList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*GlobalRoleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalRoleController
	AddHandler(ctx context.Context, name string, sync GlobalRoleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle GlobalRoleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalRoleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalRoleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalRoleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalRoleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalRoleLifecycle)
}

type globalRoleLister struct {
	controller *globalRoleController
}

func (l *globalRoleLister) List(namespace string, selector labels.Selector) (ret []*GlobalRole, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GlobalRole))
	})
	return
}

func (l *globalRoleLister) Get(namespace, name string) (*GlobalRole, error) {
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
			Group:    GlobalRoleGroupVersionKind.Group,
			Resource: "globalRole",
		}, key)
	}
	return obj.(*GlobalRole), nil
}

type globalRoleController struct {
	controller.GenericController
}

func (c *globalRoleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *globalRoleController) Lister() GlobalRoleLister {
	return &globalRoleLister{
		controller: c,
	}
}

func (c *globalRoleController) AddHandler(ctx context.Context, name string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalRole); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *globalRoleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler GlobalRoleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*GlobalRole); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type globalRoleFactory struct {
}

func (c globalRoleFactory) Object() runtime.Object {
	return &GlobalRole{}
}

func (c globalRoleFactory) List() runtime.Object {
	return &GlobalRoleList{}
}

func (s *globalRoleClient) Controller() GlobalRoleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.globalRoleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GlobalRoleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &globalRoleController{
		GenericController: genericController,
	}

	s.client.globalRoleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type globalRoleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   GlobalRoleController
}

func (s *globalRoleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *globalRoleClient) Create(o *GlobalRole) (*GlobalRole, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GlobalRole), err
}

func (s *globalRoleClient) Get(name string, opts metav1.GetOptions) (*GlobalRole, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GlobalRole), err
}

func (s *globalRoleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*GlobalRole, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*GlobalRole), err
}

func (s *globalRoleClient) Update(o *GlobalRole) (*GlobalRole, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GlobalRole), err
}

func (s *globalRoleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalRoleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *globalRoleClient) List(opts metav1.ListOptions) (*GlobalRoleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GlobalRoleList), err
}

func (s *globalRoleClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*GlobalRoleList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*GlobalRoleList), err
}

func (s *globalRoleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalRoleClient) Patch(o *GlobalRole, patchType types.PatchType, data []byte, subresources ...string) (*GlobalRole, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*GlobalRole), err
}

func (s *globalRoleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalRoleClient) AddHandler(ctx context.Context, name string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalRoleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalRoleClient) AddLifecycle(ctx context.Context, name string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *globalRoleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *globalRoleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalRoleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync GlobalRoleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *globalRoleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *globalRoleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle GlobalRoleLifecycle) {
	sync := NewGlobalRoleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
