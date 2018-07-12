package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ResourceQuotaGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ResourceQuota",
	}
	ResourceQuotaResource = metav1.APIResource{
		Name:         "resourcequotas",
		SingularName: "resourcequota",
		Namespaced:   true,

		Kind: ResourceQuotaGroupVersionKind.Kind,
	}
)

type ResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ResourceQuota
}

type ResourceQuotaHandlerFunc func(key string, obj *v1.ResourceQuota) error

type ResourceQuotaLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ResourceQuota, err error)
	Get(namespace, name string) (*v1.ResourceQuota, error)
}

type ResourceQuotaController interface {
	Informer() cache.SharedIndexInformer
	Lister() ResourceQuotaLister
	AddHandler(name string, handler ResourceQuotaHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ResourceQuotaHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ResourceQuotaInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Get(name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Update(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ResourceQuotaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ResourceQuotaController
	AddHandler(name string, sync ResourceQuotaHandlerFunc)
	AddLifecycle(name string, lifecycle ResourceQuotaLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ResourceQuotaLifecycle)
}

type resourceQuotaLister struct {
	controller *resourceQuotaController
}

func (l *resourceQuotaLister) List(namespace string, selector labels.Selector) (ret []*v1.ResourceQuota, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ResourceQuota))
	})
	return
}

func (l *resourceQuotaLister) Get(namespace, name string) (*v1.ResourceQuota, error) {
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
			Group:    ResourceQuotaGroupVersionKind.Group,
			Resource: "resourceQuota",
		}, key)
	}
	return obj.(*v1.ResourceQuota), nil
}

type resourceQuotaController struct {
	controller.GenericController
}

func (c *resourceQuotaController) Lister() ResourceQuotaLister {
	return &resourceQuotaLister{
		controller: c,
	}
}

func (c *resourceQuotaController) AddHandler(name string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.ResourceQuota))
	})
}

func (c *resourceQuotaController) AddClusterScopedHandler(name, cluster string, handler ResourceQuotaHandlerFunc) {
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

		return handler(key, obj.(*v1.ResourceQuota))
	})
}

type resourceQuotaFactory struct {
}

func (c resourceQuotaFactory) Object() runtime.Object {
	return &v1.ResourceQuota{}
}

func (c resourceQuotaFactory) List() runtime.Object {
	return &ResourceQuotaList{}
}

func (s *resourceQuotaClient) Controller() ResourceQuotaController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.resourceQuotaControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ResourceQuotaGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &resourceQuotaController{
		GenericController: genericController,
	}

	s.client.resourceQuotaControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type resourceQuotaClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ResourceQuotaController
}

func (s *resourceQuotaClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *resourceQuotaClient) Create(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Get(name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Update(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *resourceQuotaClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *resourceQuotaClient) List(opts metav1.ListOptions) (*ResourceQuotaList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ResourceQuotaList), err
}

func (s *resourceQuotaClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *resourceQuotaClient) Patch(o *v1.ResourceQuota, data []byte, subresources ...string) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *resourceQuotaClient) AddHandler(name string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *resourceQuotaClient) AddLifecycle(name string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *resourceQuotaClient) AddClusterScopedHandler(name, clusterName string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
