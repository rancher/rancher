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
	LdapConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "LdapConfig",
	}
	LdapConfigResource = metav1.APIResource{
		Name:         "ldapconfigs",
		SingularName: "ldapconfig",
		Namespaced:   false,
		Kind:         LdapConfigGroupVersionKind.Kind,
	}
)

type LdapConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LdapConfig
}

type LdapConfigHandlerFunc func(key string, obj *LdapConfig) error

type LdapConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*LdapConfig, err error)
	Get(namespace, name string) (*LdapConfig, error)
}

type LdapConfigController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() LdapConfigLister
	AddHandler(name string, handler LdapConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler LdapConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type LdapConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*LdapConfig) (*LdapConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*LdapConfig, error)
	Get(name string, opts metav1.GetOptions) (*LdapConfig, error)
	Update(*LdapConfig) (*LdapConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*LdapConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() LdapConfigController
	AddHandler(name string, sync LdapConfigHandlerFunc)
	AddLifecycle(name string, lifecycle LdapConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync LdapConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle LdapConfigLifecycle)
}

type ldapConfigLister struct {
	controller *ldapConfigController
}

func (l *ldapConfigLister) List(namespace string, selector labels.Selector) (ret []*LdapConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*LdapConfig))
	})
	return
}

func (l *ldapConfigLister) Get(namespace, name string) (*LdapConfig, error) {
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
			Group:    LdapConfigGroupVersionKind.Group,
			Resource: "ldapConfig",
		}, key)
	}
	return obj.(*LdapConfig), nil
}

type ldapConfigController struct {
	controller.GenericController
}

func (c *ldapConfigController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *ldapConfigController) Lister() LdapConfigLister {
	return &ldapConfigLister{
		controller: c,
	}
}

func (c *ldapConfigController) AddHandler(name string, handler LdapConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*LdapConfig))
	})
}

func (c *ldapConfigController) AddClusterScopedHandler(name, cluster string, handler LdapConfigHandlerFunc) {
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

		return handler(key, obj.(*LdapConfig))
	})
}

type ldapConfigFactory struct {
}

func (c ldapConfigFactory) Object() runtime.Object {
	return &LdapConfig{}
}

func (c ldapConfigFactory) List() runtime.Object {
	return &LdapConfigList{}
}

func (s *ldapConfigClient) Controller() LdapConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.ldapConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(LdapConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &ldapConfigController{
		GenericController: genericController,
	}

	s.client.ldapConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type ldapConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   LdapConfigController
}

func (s *ldapConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *ldapConfigClient) Create(o *LdapConfig) (*LdapConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) Get(name string, opts metav1.GetOptions) (*LdapConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*LdapConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) Update(o *LdapConfig) (*LdapConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *ldapConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *ldapConfigClient) List(opts metav1.ListOptions) (*LdapConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*LdapConfigList), err
}

func (s *ldapConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *ldapConfigClient) Patch(o *LdapConfig, data []byte, subresources ...string) (*LdapConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*LdapConfig), err
}

func (s *ldapConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *ldapConfigClient) AddHandler(name string, sync LdapConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *ldapConfigClient) AddLifecycle(name string, lifecycle LdapConfigLifecycle) {
	sync := NewLdapConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *ldapConfigClient) AddClusterScopedHandler(name, clusterName string, sync LdapConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *ldapConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle LdapConfigLifecycle) {
	sync := NewLdapConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
