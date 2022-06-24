package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
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
	ClusterUserAttributeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterUserAttribute",
	}
	ClusterUserAttributeResource = metav1.APIResource{
		Name:         "clusteruserattributes",
		SingularName: "clusteruserattribute",
		Namespaced:   true,

		Kind: ClusterUserAttributeGroupVersionKind.Kind,
	}

	ClusterUserAttributeGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusteruserattributes",
	}
)

func init() {
	resource.Put(ClusterUserAttributeGroupVersionResource)
}

// Deprecated: use v3.ClusterUserAttribute instead
type ClusterUserAttribute = v3.ClusterUserAttribute

func NewClusterUserAttribute(namespace, name string, obj v3.ClusterUserAttribute) *v3.ClusterUserAttribute {
	obj.APIVersion, obj.Kind = ClusterUserAttributeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterUserAttributeHandlerFunc func(key string, obj *v3.ClusterUserAttribute) (runtime.Object, error)

type ClusterUserAttributeChangeHandlerFunc func(obj *v3.ClusterUserAttribute) (runtime.Object, error)

type ClusterUserAttributeLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.ClusterUserAttribute, err error)
	Get(namespace, name string) (*v3.ClusterUserAttribute, error)
}

type ClusterUserAttributeController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterUserAttributeLister
	AddHandler(ctx context.Context, name string, handler ClusterUserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterUserAttributeHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterUserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterUserAttributeHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type ClusterUserAttributeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.ClusterUserAttribute) (*v3.ClusterUserAttribute, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterUserAttribute, error)
	Get(name string, opts metav1.GetOptions) (*v3.ClusterUserAttribute, error)
	Update(*v3.ClusterUserAttribute) (*v3.ClusterUserAttribute, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.ClusterUserAttributeList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterUserAttributeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterUserAttributeController
	AddHandler(ctx context.Context, name string, sync ClusterUserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterUserAttributeHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterUserAttributeLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterUserAttributeLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterUserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterUserAttributeHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterUserAttributeLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterUserAttributeLifecycle)
}

type clusterUserAttributeLister struct {
	ns         string
	controller *clusterUserAttributeController
}

func (l *clusterUserAttributeLister) List(namespace string, selector labels.Selector) (ret []*v3.ClusterUserAttribute, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.ClusterUserAttribute))
	})
	return
}

func (l *clusterUserAttributeLister) Get(namespace, name string) (*v3.ClusterUserAttribute, error) {
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
			Group:    ClusterUserAttributeGroupVersionKind.Group,
			Resource: ClusterUserAttributeGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.ClusterUserAttribute), nil
}

type clusterUserAttributeController struct {
	ns string
	controller.GenericController
}

func (c *clusterUserAttributeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterUserAttributeController) Lister() ClusterUserAttributeLister {
	return &clusterUserAttributeLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *clusterUserAttributeController) AddHandler(ctx context.Context, name string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterUserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterUserAttributeController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterUserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterUserAttributeController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterUserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterUserAttributeController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.ClusterUserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterUserAttributeFactory struct {
}

func (c clusterUserAttributeFactory) Object() runtime.Object {
	return &v3.ClusterUserAttribute{}
}

func (c clusterUserAttributeFactory) List() runtime.Object {
	return &v3.ClusterUserAttributeList{}
}

func (s *clusterUserAttributeClient) Controller() ClusterUserAttributeController {
	genericController := controller.NewGenericController(s.ns, ClusterUserAttributeGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(ClusterUserAttributeGroupVersionResource, ClusterUserAttributeGroupVersionKind.Kind, true))

	return &clusterUserAttributeController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type clusterUserAttributeClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterUserAttributeController
}

func (s *clusterUserAttributeClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterUserAttributeClient) Create(o *v3.ClusterUserAttribute) (*v3.ClusterUserAttribute, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) Get(name string, opts metav1.GetOptions) (*v3.ClusterUserAttribute, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.ClusterUserAttribute, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) Update(o *v3.ClusterUserAttribute) (*v3.ClusterUserAttribute, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) UpdateStatus(o *v3.ClusterUserAttribute) (*v3.ClusterUserAttribute, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterUserAttributeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterUserAttributeClient) List(opts metav1.ListOptions) (*v3.ClusterUserAttributeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.ClusterUserAttributeList), err
}

func (s *clusterUserAttributeClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.ClusterUserAttributeList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.ClusterUserAttributeList), err
}

func (s *clusterUserAttributeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterUserAttributeClient) Patch(o *v3.ClusterUserAttribute, patchType types.PatchType, data []byte, subresources ...string) (*v3.ClusterUserAttribute, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterUserAttributeClient) AddHandler(ctx context.Context, name string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterUserAttributeClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterUserAttributeClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterUserAttributeClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
