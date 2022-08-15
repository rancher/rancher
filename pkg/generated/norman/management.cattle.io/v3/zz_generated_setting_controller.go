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
	SettingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Setting",
	}
	SettingResource = metav1.APIResource{
		Name:         "settings",
		SingularName: "setting",
		Namespaced:   false,
		Kind:         SettingGroupVersionKind.Kind,
	}

	SettingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "settings",
	}
)

func init() {
	resource.Put(SettingGroupVersionResource)
}

// Deprecated: use v3.Setting instead
type Setting = v3.Setting

func NewSetting(namespace, name string, obj v3.Setting) *v3.Setting {
	obj.APIVersion, obj.Kind = SettingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SettingHandlerFunc func(key string, obj *v3.Setting) (runtime.Object, error)

type SettingChangeHandlerFunc func(obj *v3.Setting) (runtime.Object, error)

type SettingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Setting, err error)
	Get(namespace, name string) (*v3.Setting, error)
}

type SettingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SettingLister
	AddHandler(ctx context.Context, name string, handler SettingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SettingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SettingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler SettingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type SettingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Setting) (*v3.Setting, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Setting, error)
	Get(name string, opts metav1.GetOptions) (*v3.Setting, error)
	Update(*v3.Setting) (*v3.Setting, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.SettingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.SettingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SettingController
	AddHandler(ctx context.Context, name string, sync SettingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SettingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SettingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SettingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SettingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SettingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SettingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SettingLifecycle)
}

type settingLister struct {
	ns         string
	controller *settingController
}

func (l *settingLister) List(namespace string, selector labels.Selector) (ret []*v3.Setting, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Setting))
	})
	return
}

func (l *settingLister) Get(namespace, name string) (*v3.Setting, error) {
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
			Group:    SettingGroupVersionKind.Group,
			Resource: SettingGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Setting), nil
}

type settingController struct {
	ns string
	controller.GenericController
}

func (c *settingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *settingController) Lister() SettingLister {
	return &settingLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *settingController) AddHandler(ctx context.Context, name string, handler SettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Setting); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *settingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler SettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Setting); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *settingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Setting); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *settingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler SettingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Setting); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type settingFactory struct {
}

func (c settingFactory) Object() runtime.Object {
	return &v3.Setting{}
}

func (c settingFactory) List() runtime.Object {
	return &v3.SettingList{}
}

func (s *settingClient) Controller() SettingController {
	genericController := controller.NewGenericController(s.ns, SettingGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(SettingGroupVersionResource, SettingGroupVersionKind.Kind, false))

	return &settingController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type settingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SettingController
}

func (s *settingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *settingClient) Create(o *v3.Setting) (*v3.Setting, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Setting), err
}

func (s *settingClient) Get(name string, opts metav1.GetOptions) (*v3.Setting, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Setting), err
}

func (s *settingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Setting, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Setting), err
}

func (s *settingClient) Update(o *v3.Setting) (*v3.Setting, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Setting), err
}

func (s *settingClient) UpdateStatus(o *v3.Setting) (*v3.Setting, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Setting), err
}

func (s *settingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *settingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *settingClient) List(opts metav1.ListOptions) (*v3.SettingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.SettingList), err
}

func (s *settingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.SettingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.SettingList), err
}

func (s *settingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *settingClient) Patch(o *v3.Setting, patchType types.PatchType, data []byte, subresources ...string) (*v3.Setting, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Setting), err
}

func (s *settingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *settingClient) AddHandler(ctx context.Context, name string, sync SettingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *settingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SettingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *settingClient) AddLifecycle(ctx context.Context, name string, lifecycle SettingLifecycle) {
	sync := NewSettingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *settingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SettingLifecycle) {
	sync := NewSettingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *settingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SettingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *settingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SettingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *settingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SettingLifecycle) {
	sync := NewSettingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *settingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SettingLifecycle) {
	sync := NewSettingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
