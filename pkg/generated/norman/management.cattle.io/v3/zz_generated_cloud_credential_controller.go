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
	CloudCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CloudCredential",
	}
	CloudCredentialResource = metav1.APIResource{
		Name:         "cloudcredentials",
		SingularName: "cloudcredential",
		Namespaced:   true,

		Kind: CloudCredentialGroupVersionKind.Kind,
	}

	CloudCredentialGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "cloudcredentials",
	}
)

func init() {
	resource.Put(CloudCredentialGroupVersionResource)
}

// Deprecated: use v3.CloudCredential instead
type CloudCredential = v3.CloudCredential

func NewCloudCredential(namespace, name string, obj v3.CloudCredential) *v3.CloudCredential {
	obj.APIVersion, obj.Kind = CloudCredentialGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CloudCredentialHandlerFunc func(key string, obj *v3.CloudCredential) (runtime.Object, error)

type CloudCredentialChangeHandlerFunc func(obj *v3.CloudCredential) (runtime.Object, error)

type CloudCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.CloudCredential, err error)
	Get(namespace, name string) (*v3.CloudCredential, error)
}

type CloudCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CloudCredentialLister
	AddHandler(ctx context.Context, name string, handler CloudCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CloudCredentialHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CloudCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CloudCredentialHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type CloudCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.CloudCredential) (*v3.CloudCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.CloudCredential, error)
	Get(name string, opts metav1.GetOptions) (*v3.CloudCredential, error)
	Update(*v3.CloudCredential) (*v3.CloudCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.CloudCredentialList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CloudCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CloudCredentialController
	AddHandler(ctx context.Context, name string, sync CloudCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CloudCredentialHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CloudCredentialLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CloudCredentialLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CloudCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CloudCredentialHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CloudCredentialLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CloudCredentialLifecycle)
}

type cloudCredentialLister struct {
	ns         string
	controller *cloudCredentialController
}

func (l *cloudCredentialLister) List(namespace string, selector labels.Selector) (ret []*v3.CloudCredential, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.CloudCredential))
	})
	return
}

func (l *cloudCredentialLister) Get(namespace, name string) (*v3.CloudCredential, error) {
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
			Group:    CloudCredentialGroupVersionKind.Group,
			Resource: CloudCredentialGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.CloudCredential), nil
}

type cloudCredentialController struct {
	ns string
	controller.GenericController
}

func (c *cloudCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cloudCredentialController) Lister() CloudCredentialLister {
	return &cloudCredentialLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *cloudCredentialController) AddHandler(ctx context.Context, name string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CloudCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cloudCredentialController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CloudCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cloudCredentialController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CloudCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cloudCredentialController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.CloudCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type cloudCredentialFactory struct {
}

func (c cloudCredentialFactory) Object() runtime.Object {
	return &v3.CloudCredential{}
}

func (c cloudCredentialFactory) List() runtime.Object {
	return &v3.CloudCredentialList{}
}

func (s *cloudCredentialClient) Controller() CloudCredentialController {
	genericController := controller.NewGenericController(s.ns, CloudCredentialGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(CloudCredentialGroupVersionResource, CloudCredentialGroupVersionKind.Kind, true))

	return &cloudCredentialController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type cloudCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CloudCredentialController
}

func (s *cloudCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cloudCredentialClient) Create(o *v3.CloudCredential) (*v3.CloudCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.CloudCredential), err
}

func (s *cloudCredentialClient) Get(name string, opts metav1.GetOptions) (*v3.CloudCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.CloudCredential), err
}

func (s *cloudCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.CloudCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.CloudCredential), err
}

func (s *cloudCredentialClient) Update(o *v3.CloudCredential) (*v3.CloudCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.CloudCredential), err
}

func (s *cloudCredentialClient) UpdateStatus(o *v3.CloudCredential) (*v3.CloudCredential, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.CloudCredential), err
}

func (s *cloudCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cloudCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cloudCredentialClient) List(opts metav1.ListOptions) (*v3.CloudCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.CloudCredentialList), err
}

func (s *cloudCredentialClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CloudCredentialList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.CloudCredentialList), err
}

func (s *cloudCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cloudCredentialClient) Patch(o *v3.CloudCredential, patchType types.PatchType, data []byte, subresources ...string) (*v3.CloudCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.CloudCredential), err
}

func (s *cloudCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cloudCredentialClient) AddHandler(ctx context.Context, name string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cloudCredentialClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cloudCredentialClient) AddLifecycle(ctx context.Context, name string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cloudCredentialClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cloudCredentialClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
