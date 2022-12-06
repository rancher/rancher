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
	PodSecurityAdmissionConfigurationTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityAdmissionConfigurationTemplate",
	}
	PodSecurityAdmissionConfigurationTemplateResource = metav1.APIResource{
		Name:         "podsecurityadmissionconfigurationtemplates",
		SingularName: "podsecurityadmissionconfigurationtemplate",
		Namespaced:   false,
		Kind:         PodSecurityAdmissionConfigurationTemplateGroupVersionKind.Kind,
	}

	PodSecurityAdmissionConfigurationTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "podsecurityadmissionconfigurationtemplates",
	}
)

func init() {
	resource.Put(PodSecurityAdmissionConfigurationTemplateGroupVersionResource)
}

// Deprecated: use v3.PodSecurityAdmissionConfigurationTemplate instead
type PodSecurityAdmissionConfigurationTemplate = v3.PodSecurityAdmissionConfigurationTemplate

func NewPodSecurityAdmissionConfigurationTemplate(namespace, name string, obj v3.PodSecurityAdmissionConfigurationTemplate) *v3.PodSecurityAdmissionConfigurationTemplate {
	obj.APIVersion, obj.Kind = PodSecurityAdmissionConfigurationTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodSecurityAdmissionConfigurationTemplateHandlerFunc func(key string, obj *v3.PodSecurityAdmissionConfigurationTemplate) (runtime.Object, error)

type PodSecurityAdmissionConfigurationTemplateChangeHandlerFunc func(obj *v3.PodSecurityAdmissionConfigurationTemplate) (runtime.Object, error)

type PodSecurityAdmissionConfigurationTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.PodSecurityAdmissionConfigurationTemplate, err error)
	Get(namespace, name string) (*v3.PodSecurityAdmissionConfigurationTemplate, error)
}

type PodSecurityAdmissionConfigurationTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityAdmissionConfigurationTemplateLister
	AddHandler(ctx context.Context, name string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PodSecurityAdmissionConfigurationTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.PodSecurityAdmissionConfigurationTemplate) (*v3.PodSecurityAdmissionConfigurationTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PodSecurityAdmissionConfigurationTemplate, error)
	Get(name string, opts metav1.GetOptions) (*v3.PodSecurityAdmissionConfigurationTemplate, error)
	Update(*v3.PodSecurityAdmissionConfigurationTemplate) (*v3.PodSecurityAdmissionConfigurationTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.PodSecurityAdmissionConfigurationTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PodSecurityAdmissionConfigurationTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityAdmissionConfigurationTemplateController
	AddHandler(ctx context.Context, name string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle)
}

type podSecurityAdmissionConfigurationTemplateLister struct {
	ns         string
	controller *podSecurityAdmissionConfigurationTemplateController
}

func (l *podSecurityAdmissionConfigurationTemplateLister) List(namespace string, selector labels.Selector) (ret []*v3.PodSecurityAdmissionConfigurationTemplate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.PodSecurityAdmissionConfigurationTemplate))
	})
	return
}

func (l *podSecurityAdmissionConfigurationTemplateLister) Get(namespace, name string) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
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
			Group:    PodSecurityAdmissionConfigurationTemplateGroupVersionKind.Group,
			Resource: PodSecurityAdmissionConfigurationTemplateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), nil
}

type podSecurityAdmissionConfigurationTemplateController struct {
	ns string
	controller.GenericController
}

func (c *podSecurityAdmissionConfigurationTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podSecurityAdmissionConfigurationTemplateController) Lister() PodSecurityAdmissionConfigurationTemplateLister {
	return &podSecurityAdmissionConfigurationTemplateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *podSecurityAdmissionConfigurationTemplateController) AddHandler(ctx context.Context, name string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityAdmissionConfigurationTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityAdmissionConfigurationTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityAdmissionConfigurationTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityAdmissionConfigurationTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityAdmissionConfigurationTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityAdmissionConfigurationTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.PodSecurityAdmissionConfigurationTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podSecurityAdmissionConfigurationTemplateFactory struct {
}

func (c podSecurityAdmissionConfigurationTemplateFactory) Object() runtime.Object {
	return &v3.PodSecurityAdmissionConfigurationTemplate{}
}

func (c podSecurityAdmissionConfigurationTemplateFactory) List() runtime.Object {
	return &v3.PodSecurityAdmissionConfigurationTemplateList{}
}

func (s *podSecurityAdmissionConfigurationTemplateClient) Controller() PodSecurityAdmissionConfigurationTemplateController {
	genericController := controller.NewGenericController(s.ns, PodSecurityAdmissionConfigurationTemplateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PodSecurityAdmissionConfigurationTemplateGroupVersionResource, PodSecurityAdmissionConfigurationTemplateGroupVersionKind.Kind, false))

	return &podSecurityAdmissionConfigurationTemplateController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type podSecurityAdmissionConfigurationTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodSecurityAdmissionConfigurationTemplateController
}

func (s *podSecurityAdmissionConfigurationTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podSecurityAdmissionConfigurationTemplateClient) Create(o *v3.PodSecurityAdmissionConfigurationTemplate) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) Get(name string, opts metav1.GetOptions) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) Update(o *v3.PodSecurityAdmissionConfigurationTemplate) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) UpdateStatus(o *v3.PodSecurityAdmissionConfigurationTemplate) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) List(opts metav1.ListOptions) (*v3.PodSecurityAdmissionConfigurationTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplateList), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.PodSecurityAdmissionConfigurationTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplateList), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityAdmissionConfigurationTemplateClient) Patch(o *v3.PodSecurityAdmissionConfigurationTemplate, patchType types.PatchType, data []byte, subresources ...string) (*v3.PodSecurityAdmissionConfigurationTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.PodSecurityAdmissionConfigurationTemplate), err
}

func (s *podSecurityAdmissionConfigurationTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddHandler(ctx context.Context, name string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle) {
	sync := NewPodSecurityAdmissionConfigurationTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle) {
	sync := NewPodSecurityAdmissionConfigurationTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityAdmissionConfigurationTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle) {
	sync := NewPodSecurityAdmissionConfigurationTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityAdmissionConfigurationTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle) {
	sync := NewPodSecurityAdmissionConfigurationTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
