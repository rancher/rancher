package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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
	SourceCodeCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeCredential",
	}
	SourceCodeCredentialResource = metav1.APIResource{
		Name:         "sourcecodecredentials",
		SingularName: "sourcecodecredential",
		Namespaced:   true,

		Kind: SourceCodeCredentialGroupVersionKind.Kind,
	}

	SourceCodeCredentialGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "sourcecodecredentials",
	}
)

func init() {
	resource.Put(SourceCodeCredentialGroupVersionResource)
}

// Deprecated use v3.SourceCodeCredential instead
type SourceCodeCredential = v3.SourceCodeCredential

func NewSourceCodeCredential(namespace, name string, obj v3.SourceCodeCredential) *v3.SourceCodeCredential {
	obj.APIVersion, obj.Kind = SourceCodeCredentialGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeCredentialHandlerFunc func(key string, obj *v3.SourceCodeCredential) (runtime.Object, error)

type SourceCodeCredentialChangeHandlerFunc func(obj *v3.SourceCodeCredential) (runtime.Object, error)

type SourceCodeCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.SourceCodeCredential, err error)
	Get(namespace, name string) (*v3.SourceCodeCredential, error)
}

type SourceCodeCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeCredentialLister
	AddHandler(ctx context.Context, name string, handler SourceCodeCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeCredentialHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SourceCodeCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler SourceCodeCredentialHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type SourceCodeCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.SourceCodeCredential) (*v3.SourceCodeCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.SourceCodeCredential, error)
	Get(name string, opts metav1.GetOptions) (*v3.SourceCodeCredential, error)
	Update(*v3.SourceCodeCredential) (*v3.SourceCodeCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.SourceCodeCredentialList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.SourceCodeCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeCredentialController
	AddHandler(ctx context.Context, name string, sync SourceCodeCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeCredentialHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeCredentialLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SourceCodeCredentialLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SourceCodeCredentialHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeCredentialLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SourceCodeCredentialLifecycle)
}

type sourceCodeCredentialLister struct {
	ns         string
	controller *sourceCodeCredentialController
}

func (l *sourceCodeCredentialLister) List(namespace string, selector labels.Selector) (ret []*v3.SourceCodeCredential, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.SourceCodeCredential))
	})
	return
}

func (l *sourceCodeCredentialLister) Get(namespace, name string) (*v3.SourceCodeCredential, error) {
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
			Group:    SourceCodeCredentialGroupVersionKind.Group,
			Resource: SourceCodeCredentialGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.SourceCodeCredential), nil
}

type sourceCodeCredentialController struct {
	ns string
	controller.GenericController
}

func (c *sourceCodeCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeCredentialController) Lister() SourceCodeCredentialLister {
	return &sourceCodeCredentialLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *sourceCodeCredentialController) AddHandler(ctx context.Context, name string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.SourceCodeCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeCredentialController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.SourceCodeCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeCredentialController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.SourceCodeCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeCredentialController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.SourceCodeCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeCredentialFactory struct {
}

func (c sourceCodeCredentialFactory) Object() runtime.Object {
	return &v3.SourceCodeCredential{}
}

func (c sourceCodeCredentialFactory) List() runtime.Object {
	return &v3.SourceCodeCredentialList{}
}

func (s *sourceCodeCredentialClient) Controller() SourceCodeCredentialController {
	genericController := controller.NewGenericController(s.ns, SourceCodeCredentialGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(SourceCodeCredentialGroupVersionResource, SourceCodeCredentialGroupVersionKind.Kind, true))

	return &sourceCodeCredentialController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type sourceCodeCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeCredentialController
}

func (s *sourceCodeCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeCredentialClient) Create(o *v3.SourceCodeCredential) (*v3.SourceCodeCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Get(name string, opts metav1.GetOptions) (*v3.SourceCodeCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.SourceCodeCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Update(o *v3.SourceCodeCredential) (*v3.SourceCodeCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) UpdateStatus(o *v3.SourceCodeCredential) (*v3.SourceCodeCredential, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeCredentialClient) List(opts metav1.ListOptions) (*v3.SourceCodeCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.SourceCodeCredentialList), err
}

func (s *sourceCodeCredentialClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.SourceCodeCredentialList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.SourceCodeCredentialList), err
}

func (s *sourceCodeCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeCredentialClient) Patch(o *v3.SourceCodeCredential, patchType types.PatchType, data []byte, subresources ...string) (*v3.SourceCodeCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeCredentialClient) AddHandler(ctx context.Context, name string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeCredentialClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sourceCodeCredentialClient) AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeCredentialClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
