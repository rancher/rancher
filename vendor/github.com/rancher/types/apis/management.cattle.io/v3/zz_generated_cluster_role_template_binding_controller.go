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
	ClusterRoleTemplateBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRoleTemplateBinding",
	}
	ClusterRoleTemplateBindingResource = metav1.APIResource{
		Name:         "clusterroletemplatebindings",
		SingularName: "clusterroletemplatebinding",
		Namespaced:   true,

		Kind: ClusterRoleTemplateBindingGroupVersionKind.Kind,
	}

	ClusterRoleTemplateBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusterroletemplatebindings",
	}
)

func init() {
	resource.Put(ClusterRoleTemplateBindingGroupVersionResource)
}

func NewClusterRoleTemplateBinding(namespace, name string, obj ClusterRoleTemplateBinding) *ClusterRoleTemplateBinding {
	obj.APIVersion, obj.Kind = ClusterRoleTemplateBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterRoleTemplateBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRoleTemplateBinding `json:"items"`
}

type ClusterRoleTemplateBindingHandlerFunc func(key string, obj *ClusterRoleTemplateBinding) (runtime.Object, error)

type ClusterRoleTemplateBindingChangeHandlerFunc func(obj *ClusterRoleTemplateBinding) (runtime.Object, error)

type ClusterRoleTemplateBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterRoleTemplateBinding, err error)
	Get(namespace, name string) (*ClusterRoleTemplateBinding, error)
}

type ClusterRoleTemplateBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleTemplateBindingLister
	AddHandler(ctx context.Context, name string, handler ClusterRoleTemplateBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterRoleTemplateBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterRoleTemplateBindingHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRoleTemplateBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error)
	Get(name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error)
	Update(*ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRoleTemplateBindingList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterRoleTemplateBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleTemplateBindingController
	AddHandler(ctx context.Context, name string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleTemplateBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleTemplateBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleTemplateBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleTemplateBindingLifecycle)
}

type clusterRoleTemplateBindingLister struct {
	controller *clusterRoleTemplateBindingController
}

func (l *clusterRoleTemplateBindingLister) List(namespace string, selector labels.Selector) (ret []*ClusterRoleTemplateBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterRoleTemplateBinding))
	})
	return
}

func (l *clusterRoleTemplateBindingLister) Get(namespace, name string) (*ClusterRoleTemplateBinding, error) {
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
			Group:    ClusterRoleTemplateBindingGroupVersionKind.Group,
			Resource: "clusterRoleTemplateBinding",
		}, key)
	}
	return obj.(*ClusterRoleTemplateBinding), nil
}

type clusterRoleTemplateBindingController struct {
	controller.GenericController
}

func (c *clusterRoleTemplateBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRoleTemplateBindingController) Lister() ClusterRoleTemplateBindingLister {
	return &clusterRoleTemplateBindingLister{
		controller: c,
	}
}

func (c *clusterRoleTemplateBindingController) AddHandler(ctx context.Context, name string, handler ClusterRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRoleTemplateBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleTemplateBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRoleTemplateBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleTemplateBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRoleTemplateBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterRoleTemplateBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterRoleTemplateBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterRoleTemplateBindingFactory struct {
}

func (c clusterRoleTemplateBindingFactory) Object() runtime.Object {
	return &ClusterRoleTemplateBinding{}
}

func (c clusterRoleTemplateBindingFactory) List() runtime.Object {
	return &ClusterRoleTemplateBindingList{}
}

func (s *clusterRoleTemplateBindingClient) Controller() ClusterRoleTemplateBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRoleTemplateBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRoleTemplateBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRoleTemplateBindingController{
		GenericController: genericController,
	}

	s.client.clusterRoleTemplateBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRoleTemplateBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRoleTemplateBindingController
}

func (s *clusterRoleTemplateBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleTemplateBindingClient) Create(o *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) Get(name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) Update(o *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleTemplateBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRoleTemplateBindingClient) List(opts metav1.ListOptions) (*ClusterRoleTemplateBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRoleTemplateBindingList), err
}

func (s *clusterRoleTemplateBindingClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterRoleTemplateBindingList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterRoleTemplateBindingList), err
}

func (s *clusterRoleTemplateBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleTemplateBindingClient) Patch(o *ClusterRoleTemplateBinding, patchType types.PatchType, data []byte, subresources ...string) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleTemplateBindingClient) AddHandler(ctx context.Context, name string, sync ClusterRoleTemplateBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleTemplateBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterRoleTemplateBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleTemplateBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterRoleTemplateBindingLifecycle) {
	sync := NewClusterRoleTemplateBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterRoleTemplateBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterRoleTemplateBindingLifecycle) {
	sync := NewClusterRoleTemplateBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterRoleTemplateBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterRoleTemplateBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleTemplateBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterRoleTemplateBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterRoleTemplateBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterRoleTemplateBindingLifecycle) {
	sync := NewClusterRoleTemplateBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterRoleTemplateBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterRoleTemplateBindingLifecycle) {
	sync := NewClusterRoleTemplateBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
