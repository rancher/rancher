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
	PodSecurityPolicyTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicyTemplate",
	}
	PodSecurityPolicyTemplateResource = metav1.APIResource{
		Name:         "podsecuritypolicytemplates",
		SingularName: "podsecuritypolicytemplate",
		Namespaced:   false,
		Kind:         PodSecurityPolicyTemplateGroupVersionKind.Kind,
	}

	PodSecurityPolicyTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "podsecuritypolicytemplates",
	}
)

func init() {
	resource.Put(PodSecurityPolicyTemplateGroupVersionResource)
}

// Deprecated: use v3.PodSecurityPolicyTemplate instead
type PodSecurityPolicyTemplate = v3.PodSecurityPolicyTemplate

func NewPodSecurityPolicyTemplate(namespace, name string, obj v3.PodSecurityPolicyTemplate) *v3.PodSecurityPolicyTemplate {
	obj.APIVersion, obj.Kind = PodSecurityPolicyTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodSecurityPolicyTemplateHandlerFunc func(key string, obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error)

type PodSecurityPolicyTemplateChangeHandlerFunc func(obj *v3.PodSecurityPolicyTemplate) (runtime.Object, error)

type PodSecurityPolicyTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.PodSecurityPolicyTemplate, err error)
	Get(namespace, name string) (*v3.PodSecurityPolicyTemplate, error)
}

type PodSecurityPolicyTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyTemplateLister
	AddHandler(ctx context.Context, name string, handler PodSecurityPolicyTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodSecurityPolicyTemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PodSecurityPolicyTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PodSecurityPolicyTemplate, error)
	Get(name string, opts metav1.GetOptions) (*v3.PodSecurityPolicyTemplate, error)
	Update(*v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PodSecurityPolicyTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PodSecurityPolicyTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyTemplateController
	AddHandler(ctx context.Context, name string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle)
}

type podSecurityPolicyTemplateLister struct {
	ns         string
	controller *podSecurityPolicyTemplateController
}

func (l *podSecurityPolicyTemplateLister) List(namespace string, selector labels.Selector) (ret []*v3.PodSecurityPolicyTemplate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.PodSecurityPolicyTemplate))
	})
	return
}

func (l *podSecurityPolicyTemplateLister) Get(namespace, name string) (*v3.PodSecurityPolicyTemplate, error) {
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
			Group:    PodSecurityPolicyTemplateGroupVersionKind.Group,
			Resource: PodSecurityPolicyTemplateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.PodSecurityPolicyTemplate), nil
}

type podSecurityPolicyTemplateController struct {
	ns string
	controller.GenericController
}

func (c *podSecurityPolicyTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podSecurityPolicyTemplateController) Lister() PodSecurityPolicyTemplateLister {
	return &podSecurityPolicyTemplateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *podSecurityPolicyTemplateController) AddHandler(ctx context.Context, name string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityPolicyTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityPolicyTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityPolicyTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityPolicyTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podSecurityPolicyTemplateFactory struct {
}

func (c podSecurityPolicyTemplateFactory) Object() runtime.Object {
	return &v3.PodSecurityPolicyTemplate{}
}

func (c podSecurityPolicyTemplateFactory) List() runtime.Object {
	return &v3.PodSecurityPolicyTemplateList{}
}

func (s *podSecurityPolicyTemplateClient) Controller() PodSecurityPolicyTemplateController {
	genericController := controller.NewGenericController(s.ns, PodSecurityPolicyTemplateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PodSecurityPolicyTemplateGroupVersionResource, PodSecurityPolicyTemplateGroupVersionKind.Kind, false))

	return &podSecurityPolicyTemplateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type podSecurityPolicyTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodSecurityPolicyTemplateController
}

func (s *podSecurityPolicyTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyTemplateClient) Create(o *v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Get(name string, opts metav1.GetOptions) (*v3.PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Update(o *v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) UpdateStatus(o *v3.PodSecurityPolicyTemplate) (*v3.PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityPolicyTemplateClient) List(opts metav1.ListOptions) (*v3.PodSecurityPolicyTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PodSecurityPolicyTemplateList), err
}

func (s *podSecurityPolicyTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PodSecurityPolicyTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PodSecurityPolicyTemplateList), err
}

func (s *podSecurityPolicyTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyTemplateClient) Patch(o *v3.PodSecurityPolicyTemplate, patchType types.PatchType, data []byte, subresources ...string) (*v3.PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyTemplateClient) AddHandler(ctx context.Context, name string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
