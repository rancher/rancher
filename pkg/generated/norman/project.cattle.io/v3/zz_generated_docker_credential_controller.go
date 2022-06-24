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
	DockerCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DockerCredential",
	}
	DockerCredentialResource = metav1.APIResource{
		Name:         "dockercredentials",
		SingularName: "dockercredential",
		Namespaced:   true,

		Kind: DockerCredentialGroupVersionKind.Kind,
	}

	DockerCredentialGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "dockercredentials",
	}
)

func init() {
	resource.Put(DockerCredentialGroupVersionResource)
}

// Deprecated: use v3.DockerCredential instead
type DockerCredential = v3.DockerCredential

func NewDockerCredential(namespace, name string, obj v3.DockerCredential) *v3.DockerCredential {
	obj.APIVersion, obj.Kind = DockerCredentialGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DockerCredentialHandlerFunc func(key string, obj *v3.DockerCredential) (runtime.Object, error)

type DockerCredentialChangeHandlerFunc func(obj *v3.DockerCredential) (runtime.Object, error)

type DockerCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.DockerCredential, err error)
	Get(namespace, name string) (*v3.DockerCredential, error)
}

type DockerCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DockerCredentialLister
	AddHandler(ctx context.Context, name string, handler DockerCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DockerCredentialHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DockerCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler DockerCredentialHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type DockerCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.DockerCredential) (*v3.DockerCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.DockerCredential, error)
	Get(name string, opts metav1.GetOptions) (*v3.DockerCredential, error)
	Update(*v3.DockerCredential) (*v3.DockerCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.DockerCredentialList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.DockerCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DockerCredentialController
	AddHandler(ctx context.Context, name string, sync DockerCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DockerCredentialHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DockerCredentialLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DockerCredentialLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DockerCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DockerCredentialHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DockerCredentialLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DockerCredentialLifecycle)
}

type dockerCredentialLister struct {
	ns         string
	controller *dockerCredentialController
}

func (l *dockerCredentialLister) List(namespace string, selector labels.Selector) (ret []*v3.DockerCredential, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.DockerCredential))
	})
	return
}

func (l *dockerCredentialLister) Get(namespace, name string) (*v3.DockerCredential, error) {
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
			Group:    DockerCredentialGroupVersionKind.Group,
			Resource: DockerCredentialGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.DockerCredential), nil
}

type dockerCredentialController struct {
	ns string
	controller.GenericController
}

func (c *dockerCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *dockerCredentialController) Lister() DockerCredentialLister {
	return &dockerCredentialLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *dockerCredentialController) AddHandler(ctx context.Context, name string, handler DockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.DockerCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dockerCredentialController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler DockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.DockerCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dockerCredentialController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.DockerCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *dockerCredentialController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler DockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.DockerCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type dockerCredentialFactory struct {
}

func (c dockerCredentialFactory) Object() runtime.Object {
	return &v3.DockerCredential{}
}

func (c dockerCredentialFactory) List() runtime.Object {
	return &v3.DockerCredentialList{}
}

func (s *dockerCredentialClient) Controller() DockerCredentialController {
	genericController := controller.NewGenericController(s.ns, DockerCredentialGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(DockerCredentialGroupVersionResource, DockerCredentialGroupVersionKind.Kind, true))

	return &dockerCredentialController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type dockerCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DockerCredentialController
}

func (s *dockerCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *dockerCredentialClient) Create(o *v3.DockerCredential) (*v3.DockerCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.DockerCredential), err
}

func (s *dockerCredentialClient) Get(name string, opts metav1.GetOptions) (*v3.DockerCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.DockerCredential), err
}

func (s *dockerCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.DockerCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.DockerCredential), err
}

func (s *dockerCredentialClient) Update(o *v3.DockerCredential) (*v3.DockerCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.DockerCredential), err
}

func (s *dockerCredentialClient) UpdateStatus(o *v3.DockerCredential) (*v3.DockerCredential, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.DockerCredential), err
}

func (s *dockerCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *dockerCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *dockerCredentialClient) List(opts metav1.ListOptions) (*v3.DockerCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.DockerCredentialList), err
}

func (s *dockerCredentialClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.DockerCredentialList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.DockerCredentialList), err
}

func (s *dockerCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *dockerCredentialClient) Patch(o *v3.DockerCredential, patchType types.PatchType, data []byte, subresources ...string) (*v3.DockerCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.DockerCredential), err
}

func (s *dockerCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *dockerCredentialClient) AddHandler(ctx context.Context, name string, sync DockerCredentialHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *dockerCredentialClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DockerCredentialHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *dockerCredentialClient) AddLifecycle(ctx context.Context, name string, lifecycle DockerCredentialLifecycle) {
	sync := NewDockerCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *dockerCredentialClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DockerCredentialLifecycle) {
	sync := NewDockerCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *dockerCredentialClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DockerCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *dockerCredentialClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DockerCredentialHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *dockerCredentialClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DockerCredentialLifecycle) {
	sync := NewDockerCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *dockerCredentialClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DockerCredentialLifecycle) {
	sync := NewDockerCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
