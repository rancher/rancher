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

func NewClusterAlertGroup(namespace, name string, obj ClusterAlertGroup) *ClusterAlertGroup {
	obj.APIVersion, obj.Kind = ClusterAlertGroupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterAlertGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAlertGroup `json:"items"`
}

type ClusterAlertGroupHandlerFunc func(key string, obj *ClusterAlertGroup) (runtime.Object, error)

type ClusterAlertGroupChangeHandlerFunc func(obj *ClusterAlertGroup) (runtime.Object, error)

type ClusterAlertGroupLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterAlertGroup, err error)
	Get(namespace, name string) (*ClusterAlertGroup, error)
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
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterAlertGroupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterAlertGroup) (*ClusterAlertGroup, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAlertGroup, error)
	Get(name string, opts metav1.GetOptions) (*ClusterAlertGroup, error)
	Update(*ClusterAlertGroup) (*ClusterAlertGroup, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterAlertGroupList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterAlertGroupList, error)
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
	controller *clusterAlertGroupController
}

func (l *clusterAlertGroupLister) List(namespace string, selector labels.Selector) (ret []*ClusterAlertGroup, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterAlertGroup))
	})
	return
}

func (l *clusterAlertGroupLister) Get(namespace, name string) (*ClusterAlertGroup, error) {
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
			Resource: "clusterAlertGroup",
		}, key)
	}
	return obj.(*ClusterAlertGroup), nil
}

type clusterAlertGroupController struct {
	controller.GenericController
}

func (c *clusterAlertGroupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterAlertGroupController) Lister() ClusterAlertGroupLister {
	return &clusterAlertGroupLister{
		controller: c,
	}
}

func (c *clusterAlertGroupController) AddHandler(ctx context.Context, name string, handler ClusterAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterAlertGroup); ok {
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
		} else if v, ok := obj.(*ClusterAlertGroup); ok {
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
		} else if v, ok := obj.(*ClusterAlertGroup); ok && controller.ObjectInCluster(cluster, obj) {
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
		} else if v, ok := obj.(*ClusterAlertGroup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterAlertGroupFactory struct {
}

func (c clusterAlertGroupFactory) Object() runtime.Object {
	return &ClusterAlertGroup{}
}

func (c clusterAlertGroupFactory) List() runtime.Object {
	return &ClusterAlertGroupList{}
}

func (s *clusterAlertGroupClient) Controller() ClusterAlertGroupController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterAlertGroupControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterAlertGroupGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterAlertGroupController{
		GenericController: genericController,
	}

	s.client.clusterAlertGroupControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *clusterAlertGroupClient) Create(o *ClusterAlertGroup) (*ClusterAlertGroup, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) Get(name string, opts metav1.GetOptions) (*ClusterAlertGroup, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterAlertGroup, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) Update(o *ClusterAlertGroup) (*ClusterAlertGroup, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterAlertGroup), err
}

func (s *clusterAlertGroupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterAlertGroupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterAlertGroupClient) List(opts metav1.ListOptions) (*ClusterAlertGroupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterAlertGroupList), err
}

func (s *clusterAlertGroupClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterAlertGroupList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterAlertGroupList), err
}

func (s *clusterAlertGroupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterAlertGroupClient) Patch(o *ClusterAlertGroup, patchType types.PatchType, data []byte, subresources ...string) (*ClusterAlertGroup, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterAlertGroup), err
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
