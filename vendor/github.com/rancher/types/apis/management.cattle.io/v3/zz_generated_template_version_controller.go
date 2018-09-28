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
	TemplateVersionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "TemplateVersion",
	}
	TemplateVersionResource = metav1.APIResource{
		Name:         "templateversions",
		SingularName: "templateversion",
		Namespaced:   false,
		Kind:         TemplateVersionGroupVersionKind.Kind,
	}
)

type TemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateVersion
}

type TemplateVersionHandlerFunc func(key string, obj *TemplateVersion) error

type TemplateVersionLister interface {
	List(namespace string, selector labels.Selector) (ret []*TemplateVersion, err error)
	Get(namespace, name string) (*TemplateVersion, error)
}

type TemplateVersionController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TemplateVersionLister
	AddHandler(name string, handler TemplateVersionHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler TemplateVersionHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateVersionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*TemplateVersion) (*TemplateVersion, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*TemplateVersion, error)
	Get(name string, opts metav1.GetOptions) (*TemplateVersion, error)
	Update(*TemplateVersion) (*TemplateVersion, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateVersionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateVersionController
	AddHandler(name string, sync TemplateVersionHandlerFunc)
	AddLifecycle(name string, lifecycle TemplateVersionLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync TemplateVersionHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle TemplateVersionLifecycle)
}

type templateVersionLister struct {
	controller *templateVersionController
}

func (l *templateVersionLister) List(namespace string, selector labels.Selector) (ret []*TemplateVersion, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*TemplateVersion))
	})
	return
}

func (l *templateVersionLister) Get(namespace, name string) (*TemplateVersion, error) {
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
			Group:    TemplateVersionGroupVersionKind.Group,
			Resource: "templateVersion",
		}, key)
	}
	return obj.(*TemplateVersion), nil
}

type templateVersionController struct {
	controller.GenericController
}

func (c *templateVersionController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *templateVersionController) Lister() TemplateVersionLister {
	return &templateVersionLister{
		controller: c,
	}
}

func (c *templateVersionController) AddHandler(name string, handler TemplateVersionHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*TemplateVersion))
	})
}

func (c *templateVersionController) AddClusterScopedHandler(name, cluster string, handler TemplateVersionHandlerFunc) {
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

		return handler(key, obj.(*TemplateVersion))
	})
}

type templateVersionFactory struct {
}

func (c templateVersionFactory) Object() runtime.Object {
	return &TemplateVersion{}
}

func (c templateVersionFactory) List() runtime.Object {
	return &TemplateVersionList{}
}

func (s *templateVersionClient) Controller() TemplateVersionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.templateVersionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TemplateVersionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &templateVersionController{
		GenericController: genericController,
	}

	s.client.templateVersionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type templateVersionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TemplateVersionController
}

func (s *templateVersionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *templateVersionClient) Create(o *TemplateVersion) (*TemplateVersion, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) Get(name string, opts metav1.GetOptions) (*TemplateVersion, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*TemplateVersion, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) Update(o *TemplateVersion) (*TemplateVersion, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateVersionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *templateVersionClient) List(opts metav1.ListOptions) (*TemplateVersionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateVersionList), err
}

func (s *templateVersionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateVersionClient) Patch(o *TemplateVersion, data []byte, subresources ...string) (*TemplateVersion, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*TemplateVersion), err
}

func (s *templateVersionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateVersionClient) AddHandler(name string, sync TemplateVersionHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *templateVersionClient) AddLifecycle(name string, lifecycle TemplateVersionLifecycle) {
	sync := NewTemplateVersionLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *templateVersionClient) AddClusterScopedHandler(name, clusterName string, sync TemplateVersionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *templateVersionClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle TemplateVersionLifecycle) {
	sync := NewTemplateVersionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
