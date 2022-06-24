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
	PreferenceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Preference",
	}
	PreferenceResource = metav1.APIResource{
		Name:         "preferences",
		SingularName: "preference",
		Namespaced:   true,

		Kind: PreferenceGroupVersionKind.Kind,
	}

	PreferenceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "preferences",
	}
)

func init() {
	resource.Put(PreferenceGroupVersionResource)
}

// Deprecated: use v3.Preference instead
type Preference = v3.Preference

func NewPreference(namespace, name string, obj v3.Preference) *v3.Preference {
	obj.APIVersion, obj.Kind = PreferenceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PreferenceHandlerFunc func(key string, obj *v3.Preference) (runtime.Object, error)

type PreferenceChangeHandlerFunc func(obj *v3.Preference) (runtime.Object, error)

type PreferenceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Preference, err error)
	Get(namespace, name string) (*v3.Preference, error)
}

type PreferenceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PreferenceLister
	AddHandler(ctx context.Context, name string, handler PreferenceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PreferenceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PreferenceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PreferenceHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PreferenceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Preference) (*v3.Preference, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Preference, error)
	Get(name string, opts metav1.GetOptions) (*v3.Preference, error)
	Update(*v3.Preference) (*v3.Preference, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PreferenceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PreferenceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PreferenceController
	AddHandler(ctx context.Context, name string, sync PreferenceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PreferenceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PreferenceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PreferenceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PreferenceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PreferenceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PreferenceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PreferenceLifecycle)
}

type preferenceLister struct {
	ns         string
	controller *preferenceController
}

func (l *preferenceLister) List(namespace string, selector labels.Selector) (ret []*v3.Preference, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Preference))
	})
	return
}

func (l *preferenceLister) Get(namespace, name string) (*v3.Preference, error) {
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
			Group:    PreferenceGroupVersionKind.Group,
			Resource: PreferenceGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Preference), nil
}

type preferenceController struct {
	ns string
	controller.GenericController
}

func (c *preferenceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *preferenceController) Lister() PreferenceLister {
	return &preferenceLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *preferenceController) AddHandler(ctx context.Context, name string, handler PreferenceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Preference); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *preferenceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PreferenceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Preference); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *preferenceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PreferenceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Preference); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *preferenceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PreferenceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Preference); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type preferenceFactory struct {
}

func (c preferenceFactory) Object() runtime.Object {
	return &v3.Preference{}
}

func (c preferenceFactory) List() runtime.Object {
	return &v3.PreferenceList{}
}

func (s *preferenceClient) Controller() PreferenceController {
	genericController := controller.NewGenericController(s.ns, PreferenceGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PreferenceGroupVersionResource, PreferenceGroupVersionKind.Kind, true))

	return &preferenceController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type preferenceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PreferenceController
}

func (s *preferenceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *preferenceClient) Create(o *v3.Preference) (*v3.Preference, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Preference), err
}

func (s *preferenceClient) Get(name string, opts metav1.GetOptions) (*v3.Preference, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Preference), err
}

func (s *preferenceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Preference, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Preference), err
}

func (s *preferenceClient) Update(o *v3.Preference) (*v3.Preference, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Preference), err
}

func (s *preferenceClient) UpdateStatus(o *v3.Preference) (*v3.Preference, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Preference), err
}

func (s *preferenceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *preferenceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *preferenceClient) List(opts metav1.ListOptions) (*v3.PreferenceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PreferenceList), err
}

func (s *preferenceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PreferenceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PreferenceList), err
}

func (s *preferenceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *preferenceClient) Patch(o *v3.Preference, patchType types.PatchType, data []byte, subresources ...string) (*v3.Preference, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Preference), err
}

func (s *preferenceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *preferenceClient) AddHandler(ctx context.Context, name string, sync PreferenceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *preferenceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PreferenceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *preferenceClient) AddLifecycle(ctx context.Context, name string, lifecycle PreferenceLifecycle) {
	sync := NewPreferenceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *preferenceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PreferenceLifecycle) {
	sync := NewPreferenceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *preferenceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PreferenceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *preferenceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PreferenceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *preferenceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PreferenceLifecycle) {
	sync := NewPreferenceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *preferenceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PreferenceLifecycle) {
	sync := NewPreferenceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
