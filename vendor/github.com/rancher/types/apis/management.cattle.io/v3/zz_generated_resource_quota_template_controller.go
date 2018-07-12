package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ResourceQuotaTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ResourceQuotaTemplate",
	}
	ResourceQuotaTemplateResource = metav1.APIResource{
		Name:         "resourcequotatemplates",
		SingularName: "resourcequotatemplate",
		Namespaced:   true,

		Kind: ResourceQuotaTemplateGroupVersionKind.Kind,
	}
)

type ResourceQuotaTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceQuotaTemplate
}

type ResourceQuotaTemplateHandlerFunc func(key string, obj *ResourceQuotaTemplate) error

type ResourceQuotaTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*ResourceQuotaTemplate, err error)
	Get(namespace, name string) (*ResourceQuotaTemplate, error)
}

type ResourceQuotaTemplateController interface {
	Informer() cache.SharedIndexInformer
	Lister() ResourceQuotaTemplateLister
	AddHandler(name string, handler ResourceQuotaTemplateHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ResourceQuotaTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ResourceQuotaTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ResourceQuotaTemplate) (*ResourceQuotaTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ResourceQuotaTemplate, error)
	Get(name string, opts metav1.GetOptions) (*ResourceQuotaTemplate, error)
	Update(*ResourceQuotaTemplate) (*ResourceQuotaTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ResourceQuotaTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ResourceQuotaTemplateController
	AddHandler(name string, sync ResourceQuotaTemplateHandlerFunc)
	AddLifecycle(name string, lifecycle ResourceQuotaTemplateLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ResourceQuotaTemplateHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ResourceQuotaTemplateLifecycle)
}

type resourceQuotaTemplateLister struct {
	controller *resourceQuotaTemplateController
}

func (l *resourceQuotaTemplateLister) List(namespace string, selector labels.Selector) (ret []*ResourceQuotaTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ResourceQuotaTemplate))
	})
	return
}

func (l *resourceQuotaTemplateLister) Get(namespace, name string) (*ResourceQuotaTemplate, error) {
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
			Group:    ResourceQuotaTemplateGroupVersionKind.Group,
			Resource: "resourceQuotaTemplate",
		}, key)
	}
	return obj.(*ResourceQuotaTemplate), nil
}

type resourceQuotaTemplateController struct {
	controller.GenericController
}

func (c *resourceQuotaTemplateController) Lister() ResourceQuotaTemplateLister {
	return &resourceQuotaTemplateLister{
		controller: c,
	}
}

func (c *resourceQuotaTemplateController) AddHandler(name string, handler ResourceQuotaTemplateHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ResourceQuotaTemplate))
	})
}

func (c *resourceQuotaTemplateController) AddClusterScopedHandler(name, cluster string, handler ResourceQuotaTemplateHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*ResourceQuotaTemplate))
	})
}

type resourceQuotaTemplateFactory struct {
}

func (c resourceQuotaTemplateFactory) Object() runtime.Object {
	return &ResourceQuotaTemplate{}
}

func (c resourceQuotaTemplateFactory) List() runtime.Object {
	return &ResourceQuotaTemplateList{}
}

func (s *resourceQuotaTemplateClient) Controller() ResourceQuotaTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.resourceQuotaTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ResourceQuotaTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &resourceQuotaTemplateController{
		GenericController: genericController,
	}

	s.client.resourceQuotaTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type resourceQuotaTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ResourceQuotaTemplateController
}

func (s *resourceQuotaTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *resourceQuotaTemplateClient) Create(o *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ResourceQuotaTemplate), err
}

func (s *resourceQuotaTemplateClient) Get(name string, opts metav1.GetOptions) (*ResourceQuotaTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ResourceQuotaTemplate), err
}

func (s *resourceQuotaTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ResourceQuotaTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ResourceQuotaTemplate), err
}

func (s *resourceQuotaTemplateClient) Update(o *ResourceQuotaTemplate) (*ResourceQuotaTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ResourceQuotaTemplate), err
}

func (s *resourceQuotaTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *resourceQuotaTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *resourceQuotaTemplateClient) List(opts metav1.ListOptions) (*ResourceQuotaTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ResourceQuotaTemplateList), err
}

func (s *resourceQuotaTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *resourceQuotaTemplateClient) Patch(o *ResourceQuotaTemplate, data []byte, subresources ...string) (*ResourceQuotaTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ResourceQuotaTemplate), err
}

func (s *resourceQuotaTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *resourceQuotaTemplateClient) AddHandler(name string, sync ResourceQuotaTemplateHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *resourceQuotaTemplateClient) AddLifecycle(name string, lifecycle ResourceQuotaTemplateLifecycle) {
	sync := NewResourceQuotaTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *resourceQuotaTemplateClient) AddClusterScopedHandler(name, clusterName string, sync ResourceQuotaTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *resourceQuotaTemplateClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ResourceQuotaTemplateLifecycle) {
	sync := NewResourceQuotaTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
