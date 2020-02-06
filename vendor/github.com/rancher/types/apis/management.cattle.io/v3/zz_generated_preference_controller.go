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

func NewPreference(namespace, name string, obj Preference) *Preference {
	obj.APIVersion, obj.Kind = PreferenceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PreferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Preference `json:"items"`
}

type PreferenceHandlerFunc func(key string, obj *Preference) (runtime.Object, error)

type PreferenceChangeHandlerFunc func(obj *Preference) (runtime.Object, error)

type PreferenceLister interface {
	List(namespace string, selector labels.Selector) (ret []*Preference, err error)
	Get(namespace, name string) (*Preference, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PreferenceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Preference) (*Preference, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Preference, error)
	Get(name string, opts metav1.GetOptions) (*Preference, error)
	Update(*Preference) (*Preference, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PreferenceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*PreferenceList, error)
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
	controller *preferenceController
}

func (l *preferenceLister) List(namespace string, selector labels.Selector) (ret []*Preference, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Preference))
	})
	return
}

func (l *preferenceLister) Get(namespace, name string) (*Preference, error) {
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
			Resource: "preference",
		}, key)
	}
	return obj.(*Preference), nil
}

type preferenceController struct {
	controller.GenericController
}

func (c *preferenceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *preferenceController) Lister() PreferenceLister {
	return &preferenceLister{
		controller: c,
	}
}

func (c *preferenceController) AddHandler(ctx context.Context, name string, handler PreferenceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Preference); ok {
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
		} else if v, ok := obj.(*Preference); ok {
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
		} else if v, ok := obj.(*Preference); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*Preference); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type preferenceFactory struct {
}

func (c preferenceFactory) Object() runtime.Object {
	return &Preference{}
}

func (c preferenceFactory) List() runtime.Object {
	return &PreferenceList{}
}

func (s *preferenceClient) Controller() PreferenceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.preferenceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PreferenceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &preferenceController{
		GenericController: genericController,
	}

	s.client.preferenceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *preferenceClient) Create(o *Preference) (*Preference, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Preference), err
}

func (s *preferenceClient) Get(name string, opts metav1.GetOptions) (*Preference, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Preference), err
}

func (s *preferenceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Preference, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Preference), err
}

func (s *preferenceClient) Update(o *Preference) (*Preference, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Preference), err
}

func (s *preferenceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *preferenceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *preferenceClient) List(opts metav1.ListOptions) (*PreferenceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PreferenceList), err
}

func (s *preferenceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*PreferenceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*PreferenceList), err
}

func (s *preferenceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *preferenceClient) Patch(o *Preference, patchType types.PatchType, data []byte, subresources ...string) (*Preference, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Preference), err
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
