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
	SourceCodeProviderGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeProvider",
	}
	SourceCodeProviderResource = metav1.APIResource{
		Name:         "sourcecodeproviders",
		SingularName: "sourcecodeprovider",
		Namespaced:   false,
		Kind:         SourceCodeProviderGroupVersionKind.Kind,
	}
)

type SourceCodeProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeProvider
}

type SourceCodeProviderHandlerFunc func(key string, obj *SourceCodeProvider) error

type SourceCodeProviderLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeProvider, err error)
	Get(namespace, name string) (*SourceCodeProvider, error)
}

type SourceCodeProviderController interface {
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeProviderLister
	AddHandler(name string, handler SourceCodeProviderHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler SourceCodeProviderHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeProviderInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeProvider) (*SourceCodeProvider, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeProvider, error)
	Update(*SourceCodeProvider) (*SourceCodeProvider, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeProviderController
	AddHandler(name string, sync SourceCodeProviderHandlerFunc)
	AddLifecycle(name string, lifecycle SourceCodeProviderLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync SourceCodeProviderHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeProviderLifecycle)
}

type sourceCodeProviderLister struct {
	controller *sourceCodeProviderController
}

func (l *sourceCodeProviderLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeProvider, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeProvider))
	})
	return
}

func (l *sourceCodeProviderLister) Get(namespace, name string) (*SourceCodeProvider, error) {
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
			Group:    SourceCodeProviderGroupVersionKind.Group,
			Resource: "sourceCodeProvider",
		}, key)
	}
	return obj.(*SourceCodeProvider), nil
}

type sourceCodeProviderController struct {
	controller.GenericController
}

func (c *sourceCodeProviderController) Lister() SourceCodeProviderLister {
	return &sourceCodeProviderLister{
		controller: c,
	}
}

func (c *sourceCodeProviderController) AddHandler(name string, handler SourceCodeProviderHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*SourceCodeProvider))
	})
}

func (c *sourceCodeProviderController) AddClusterScopedHandler(name, cluster string, handler SourceCodeProviderHandlerFunc) {
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

		return handler(key, obj.(*SourceCodeProvider))
	})
}

type sourceCodeProviderFactory struct {
}

func (c sourceCodeProviderFactory) Object() runtime.Object {
	return &SourceCodeProvider{}
}

func (c sourceCodeProviderFactory) List() runtime.Object {
	return &SourceCodeProviderList{}
}

func (s *sourceCodeProviderClient) Controller() SourceCodeProviderController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeProviderControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeProviderGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeProviderController{
		GenericController: genericController,
	}

	s.client.sourceCodeProviderControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeProviderClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeProviderController
}

func (s *sourceCodeProviderClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeProviderClient) Create(o *SourceCodeProvider) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Get(name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Update(o *SourceCodeProvider) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeProviderClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeProviderClient) List(opts metav1.ListOptions) (*SourceCodeProviderList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeProviderList), err
}

func (s *sourceCodeProviderClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeProviderClient) Patch(o *SourceCodeProvider, data []byte, subresources ...string) (*SourceCodeProvider, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*SourceCodeProvider), err
}

func (s *sourceCodeProviderClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeProviderClient) AddHandler(name string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *sourceCodeProviderClient) AddLifecycle(name string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedHandler(name, clusterName string, sync SourceCodeProviderHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *sourceCodeProviderClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeProviderLifecycle) {
	sync := NewSourceCodeProviderLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
