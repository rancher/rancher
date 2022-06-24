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
	ClusterAlertGroupGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterAlertGroup",
	}
	ClusterAlertGroupResource = metav1.APIResource{
		Name:         "clusteralertgroups",
		SingularName: "clusteralertgroup",
		Namespaced:   true,

		Kind: ClusterAlertGroupGroupVersionKind.Kind,
	}

	ClusterAlertGroupGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusteralertgroups",
	}
)

func init() {
	resource.Put(ClusterAlertGroupGroupVersionResource)
}

// Deprecated: use v3.ClusterAlertGroup instead
type ClusterAlertGroup = v3.ClusterAlertGroup

func NewClusterAlertGroup(namespace, name string, obj v3.ClusterAlertGroup) *v3.ClusterAlertGroup {
	obj.APIVersion, obj.Kind = ClusterAlertGroupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterAlertGroupHandlerFunc func(key string, obj *v3.ClusterAlertGroup) (runtime.Object, error)

type ClusterAlertGroupChangeHandlerFunc func(obj *v3.ClusterAlertGroup) (runtime.Object, error)

type ClusterAlertGroupLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ClusterAlertGroup, err error)
	Get(namespace, name string) (*v3.ClusterAlertGroup, error)
}

type ClusterAlertGroupController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterAlertGroupLister
	AddHandler(ctx context.Context, name string, handler ClusterAlertGroupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertGroupHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterAlertGroupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterAlertGroupHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterAlertGroupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ClusterAlertGroup) (*v3.ClusterAlertGroup, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterAlertGroup, error)
	Get(name string, opts metav1.GetOptions) (*v3.ClusterAlertGroup, error)
	Update(*v3.ClusterAlertGroup) (*v3.ClusterAlertGroup, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ClusterAlertGroupList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterAlertGroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterAlertGroupController
	AddHandler(ctx context.Context, name string, sync ClusterAlertGroupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertGroupHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterAlertGroupLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAlertGroupLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAlertGroupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAlertGroupHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAlertGroupLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAlertGroupLifecycle)
}

type clusterAlertGroupLister struct {
	ns         string
	controller *clusterAlertGroupController
}

func (l *clusterAlertGroupLister) List(namespace string, selector labels.Selector) (ret []*v3.ClusterAlertGroup, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ClusterAlertGroup))
	})
	return
}

func (l *clusterAlertGroupLister) Get(namespace, name string) (*v3.ClusterAlertGroup, error) {
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
			Group:    ClusterAlertGroupGroupVersionKind.Group,
			Resource: ClusterAlertGroupGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ClusterAlertGroup), nil
}

type clusterAlertGroupController struct {
	ns string
	controller.GenericController
}

func (c *clusterAlertGroupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterAlertGroupController) Lister() ClusterAlertGroupLister {
	return &clusterAlertGroupLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterAlertGroupController) AddHandler(ctx context.Context, name string, handler ClusterAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertGroup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertGroupController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertGroup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertGroupController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertGroup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterAlertGroupController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterAlertGroup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterAlertGroupFactory struct {
}

func (c clusterAlertGroupFactory) Object() runtime.Object {
	return &v3.ClusterAlertGroup{}
}

func (c clusterAlertGroupFactory) List() runtime.Object {
	return &v3.ClusterAlertGroupList{}
}

func (s *clusterAlertGroupClient) Controller() ClusterAlertGroupController {
	genericController := controller.NewGenericController(s.ns, ClusterAlertGroupGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterAlertGroupGroupVersionResource, ClusterAlertGroupGroupVersionKind.Kind, true))

	return &clusterAlertGroupController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterAlertGroupClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterAlertGroupController
}

func (s *clusterAlertGroupClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterAlertGroupClient) Create(o *v3.ClusterAlertGroup) (*v3.ClusterAlertGroup, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) Get(name string, opts metav1.GetOptions) (*v3.ClusterAlertGroup, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterAlertGroup, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) Update(o *v3.ClusterAlertGroup) (*v3.ClusterAlertGroup, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) UpdateStatus(o *v3.ClusterAlertGroup) (*v3.ClusterAlertGroup, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterAlertGroupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterAlertGroupClient) List(opts metav1.ListOptions) (*v3.ClusterAlertGroupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ClusterAlertGroupList), err
}

func (s *clusterAlertGroupClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterAlertGroupList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ClusterAlertGroupList), err
}

func (s *clusterAlertGroupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAlertGroupClient) Patch(o *v3.ClusterAlertGroup, patchType types.PatchType, data []byte, subresources ...string) (*v3.ClusterAlertGroup, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterAlertGroupClient) AddHandler(ctx context.Context, name string, sync ClusterAlertGroupHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAlertGroupClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterAlertGroupHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAlertGroupClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterAlertGroupLifecycle) {
	sync := NewClusterAlertGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterAlertGroupClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterAlertGroupLifecycle) {
	sync := NewClusterAlertGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterAlertGroupClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterAlertGroupHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAlertGroupClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterAlertGroupHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterAlertGroupClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterAlertGroupLifecycle) {
	sync := NewClusterAlertGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterAlertGroupClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterAlertGroupLifecycle) {
	sync := NewClusterAlertGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
