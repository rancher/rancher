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
)

type RoleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RoleTemplate
}

type RoleTemplateHandlerFunc func(key string, obj *RoleTemplate) error

type RoleTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*RoleTemplate, err error)
	Get(namespace, name string) (*RoleTemplate, error)
}

type RoleTemplateController interface {
	Informer() cache.SharedIndexInformer
	Lister() RoleTemplateLister
	AddHandler(name string, handler RoleTemplateHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler RoleTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RoleTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*RoleTemplate) (*RoleTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RoleTemplate, error)
	Get(name string, opts metav1.GetOptions) (*RoleTemplate, error)
	Update(*RoleTemplate) (*RoleTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RoleTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleTemplateController
	AddHandler(name string, sync RoleTemplateHandlerFunc)
	AddLifecycle(name string, lifecycle RoleTemplateLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync RoleTemplateHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle RoleTemplateLifecycle)
}

type roleTemplateLister struct {
	controller *roleTemplateController
}

func (l *roleTemplateLister) List(namespace string, selector labels.Selector) (ret []*RoleTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*RoleTemplate))
	})
	return
}

func (l *roleTemplateLister) Get(namespace, name string) (*RoleTemplate, error) {
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
			Resource: "roleTemplate",
		}, name)
	}
	return obj.(*RoleTemplate), nil
}

type roleTemplateController struct {
	controller.GenericController
}

func (c *roleTemplateController) Lister() RoleTemplateLister {
	return &roleTemplateLister{
		controller: c,
	}
}

func (c *roleTemplateController) AddHandler(name string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*RoleTemplate))
	})
}

func (c *roleTemplateController) AddClusterScopedHandler(name, cluster string, handler RoleTemplateHandlerFunc) {
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

		return handler(key, obj.(*RoleTemplate))
	})
}

type roleTemplateFactory struct {
}

func (c roleTemplateFactory) Object() runtime.Object {
	return &RoleTemplate{}
}

func (c roleTemplateFactory) List() runtime.Object {
	return &RoleTemplateList{}
}

func (s *roleTemplateClient) Controller() RoleTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.roleTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RoleTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &roleTemplateController{
		GenericController: genericController,
	}

	s.client.roleTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *roleTemplateClient) Create(o *RoleTemplate) (*RoleTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) Get(name string, opts metav1.GetOptions) (*RoleTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RoleTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) Update(o *RoleTemplate) (*RoleTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *roleTemplateClient) List(opts metav1.ListOptions) (*RoleTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RoleTemplateList), err
}

func (s *roleTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *roleTemplateClient) Patch(o *RoleTemplate, data []byte, subresources ...string) (*RoleTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleTemplateClient) AddHandler(name string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *roleTemplateClient) AddLifecycle(name string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *roleTemplateClient) AddClusterScopedHandler(name, clusterName string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
