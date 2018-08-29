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
	SourceCodeCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeCredential",
	}
	SourceCodeCredentialResource = metav1.APIResource{
		Name:         "sourcecodecredentials",
		SingularName: "sourcecodecredential",
		Namespaced:   true,

		Kind: SourceCodeCredentialGroupVersionKind.Kind,
	}
)

type SourceCodeCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeCredential
}

type SourceCodeCredentialHandlerFunc func(key string, obj *SourceCodeCredential) error

type SourceCodeCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeCredential, err error)
	Get(namespace, name string) (*SourceCodeCredential, error)
}

type SourceCodeCredentialController interface {
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeCredentialLister
	AddHandler(name string, handler SourceCodeCredentialHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler SourceCodeCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeCredential) (*SourceCodeCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeCredential, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeCredential, error)
	Update(*SourceCodeCredential) (*SourceCodeCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeCredentialController
	AddHandler(name string, sync SourceCodeCredentialHandlerFunc)
	AddLifecycle(name string, lifecycle SourceCodeCredentialLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync SourceCodeCredentialHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeCredentialLifecycle)
}

type sourceCodeCredentialLister struct {
	controller *sourceCodeCredentialController
}

func (l *sourceCodeCredentialLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeCredential))
	})
	return
}

func (l *sourceCodeCredentialLister) Get(namespace, name string) (*SourceCodeCredential, error) {
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
			Group:    SourceCodeCredentialGroupVersionKind.Group,
			Resource: "sourceCodeCredential",
		}, key)
	}
	return obj.(*SourceCodeCredential), nil
}

type sourceCodeCredentialController struct {
	controller.GenericController
}

func (c *sourceCodeCredentialController) Lister() SourceCodeCredentialLister {
	return &sourceCodeCredentialLister{
		controller: c,
	}
}

func (c *sourceCodeCredentialController) AddHandler(name string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*SourceCodeCredential))
	})
}

func (c *sourceCodeCredentialController) AddClusterScopedHandler(name, cluster string, handler SourceCodeCredentialHandlerFunc) {
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

		return handler(key, obj.(*SourceCodeCredential))
	})
}

type sourceCodeCredentialFactory struct {
}

func (c sourceCodeCredentialFactory) Object() runtime.Object {
	return &SourceCodeCredential{}
}

func (c sourceCodeCredentialFactory) List() runtime.Object {
	return &SourceCodeCredentialList{}
}

func (s *sourceCodeCredentialClient) Controller() SourceCodeCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeCredentialController{
		GenericController: genericController,
	}

	s.client.sourceCodeCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeCredentialController
}

func (s *sourceCodeCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeCredentialClient) Create(o *SourceCodeCredential) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Get(name string, opts metav1.GetOptions) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Update(o *SourceCodeCredential) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeCredentialClient) List(opts metav1.ListOptions) (*SourceCodeCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeCredentialList), err
}

func (s *sourceCodeCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeCredentialClient) Patch(o *SourceCodeCredential, data []byte, subresources ...string) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeCredentialClient) AddHandler(name string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *sourceCodeCredentialClient) AddLifecycle(name string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedHandler(name, clusterName string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
