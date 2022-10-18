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
	RoleTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RoleTemplate",
	}
	RoleTemplateResource = metav1.APIResource{
		Name:         "roletemplates",
		SingularName: "roletemplate",
		Namespaced:   false,
		Kind:         RoleTemplateGroupVersionKind.Kind,
	}

	RoleTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "roletemplates",
	}
)

func init() {
	resource.Put(RoleTemplateGroupVersionResource)
}

// Deprecated: use v3.RoleTemplate instead
type RoleTemplate = v3.RoleTemplate

func NewRoleTemplate(namespace, name string, obj v3.RoleTemplate) *v3.RoleTemplate {
	obj.APIVersion, obj.Kind = RoleTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RoleTemplateHandlerFunc func(key string, obj *v3.RoleTemplate) (runtime.Object, error)

type RoleTemplateChangeHandlerFunc func(obj *v3.RoleTemplate) (runtime.Object, error)

type RoleTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.RoleTemplate, err error)
	Get(namespace, name string) (*v3.RoleTemplate, error)
}

type RoleTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RoleTemplateLister
	AddHandler(ctx context.Context, name string, handler RoleTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RoleTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RoleTemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type RoleTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.RoleTemplate) (*v3.RoleTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RoleTemplate, error)
	Get(name string, opts metav1.GetOptions) (*v3.RoleTemplate, error)
	Update(*v3.RoleTemplate) (*v3.RoleTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.RoleTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RoleTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleTemplateController
	AddHandler(ctx context.Context, name string, sync RoleTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RoleTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleTemplateLifecycle)
}

type roleTemplateLister struct {
	ns         string
	controller *roleTemplateController
}

func (l *roleTemplateLister) List(namespace string, selector labels.Selector) (ret []*v3.RoleTemplate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.RoleTemplate))
	})
	return
}

func (l *roleTemplateLister) Get(namespace, name string) (*v3.RoleTemplate, error) {
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
			Group:    RoleTemplateGroupVersionKind.Group,
			Resource: RoleTemplateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.RoleTemplate), nil
}

type roleTemplateController struct {
	ns string
	controller.GenericController
}

func (c *roleTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *roleTemplateController) Lister() RoleTemplateLister {
	return &roleTemplateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *roleTemplateController) AddHandler(ctx context.Context, name string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RoleTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RoleTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RoleTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.RoleTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type roleTemplateFactory struct {
}

func (c roleTemplateFactory) Object() runtime.Object {
	return &v3.RoleTemplate{}
}

func (c roleTemplateFactory) List() runtime.Object {
	return &v3.RoleTemplateList{}
}

func (s *roleTemplateClient) Controller() RoleTemplateController {
	genericController := controller.NewGenericController(s.ns, RoleTemplateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(RoleTemplateGroupVersionResource, RoleTemplateGroupVersionKind.Kind, false))

	return &roleTemplateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type roleTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RoleTemplateController
}

func (s *roleTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *roleTemplateClient) Create(o *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.RoleTemplate), err
}

func (s *roleTemplateClient) Get(name string, opts metav1.GetOptions) (*v3.RoleTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.RoleTemplate), err
}

func (s *roleTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.RoleTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.RoleTemplate), err
}

func (s *roleTemplateClient) Update(o *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.RoleTemplate), err
}

func (s *roleTemplateClient) UpdateStatus(o *v3.RoleTemplate) (*v3.RoleTemplate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.RoleTemplate), err
}

func (s *roleTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *roleTemplateClient) List(opts metav1.ListOptions) (*v3.RoleTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.RoleTemplateList), err
}

func (s *roleTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.RoleTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.RoleTemplateList), err
}

func (s *roleTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *roleTemplateClient) Patch(o *v3.RoleTemplate, patchType types.PatchType, data []byte, subresources ...string) (*v3.RoleTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.RoleTemplate), err
}

func (s *roleTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleTemplateClient) AddHandler(ctx context.Context, name string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
