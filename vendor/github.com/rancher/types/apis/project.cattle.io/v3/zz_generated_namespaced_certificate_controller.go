package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	NamespacedCertificateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedCertificate",
	}
	NamespacedCertificateResource = metav1.APIResource{
		Name:         "namespacedcertificates",
		SingularName: "namespacedcertificate",
		Namespaced:   true,

		Kind: NamespacedCertificateGroupVersionKind.Kind,
	}
)

type NamespacedCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedCertificate
}

type NamespacedCertificateHandlerFunc func(key string, obj *NamespacedCertificate) error

type NamespacedCertificateLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedCertificate, err error)
	Get(namespace, name string) (*NamespacedCertificate, error)
}

type NamespacedCertificateController interface {
	Informer() cache.SharedIndexInformer
	Lister() NamespacedCertificateLister
	AddHandler(name string, handler NamespacedCertificateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedCertificateInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*NamespacedCertificate) (*NamespacedCertificate, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*NamespacedCertificate, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedCertificate, error)
	Update(*NamespacedCertificate) (*NamespacedCertificate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedCertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedCertificateController
	AddHandler(name string, sync NamespacedCertificateHandlerFunc)
	AddLifecycle(name string, lifecycle NamespacedCertificateLifecycle)
}

type namespacedCertificateLister struct {
	controller *namespacedCertificateController
}

func (l *namespacedCertificateLister) List(namespace string, selector labels.Selector) (ret []*NamespacedCertificate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedCertificate))
	})
	return
}

func (l *namespacedCertificateLister) Get(namespace, name string) (*NamespacedCertificate, error) {
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
			Group:    NamespacedCertificateGroupVersionKind.Group,
			Resource: "namespacedCertificate",
		}, name)
	}
	return obj.(*NamespacedCertificate), nil
}

type namespacedCertificateController struct {
	controller.GenericController
}

func (c *namespacedCertificateController) Lister() NamespacedCertificateLister {
	return &namespacedCertificateLister{
		controller: c,
	}
}

func (c *namespacedCertificateController) AddHandler(name string, handler NamespacedCertificateHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NamespacedCertificate))
	})
}

type namespacedCertificateFactory struct {
}

func (c namespacedCertificateFactory) Object() runtime.Object {
	return &NamespacedCertificate{}
}

func (c namespacedCertificateFactory) List() runtime.Object {
	return &NamespacedCertificateList{}
}

func (s *namespacedCertificateClient) Controller() NamespacedCertificateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedCertificateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedCertificateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedCertificateController{
		GenericController: genericController,
	}

	s.client.namespacedCertificateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedCertificateClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   NamespacedCertificateController
}

func (s *namespacedCertificateClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *namespacedCertificateClient) Create(o *NamespacedCertificate) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Get(name string, opts metav1.GetOptions) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Update(o *NamespacedCertificate) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedCertificateClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *namespacedCertificateClient) List(opts metav1.ListOptions) (*NamespacedCertificateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedCertificateList), err
}

func (s *namespacedCertificateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedCertificateClient) Patch(o *NamespacedCertificate, data []byte, subresources ...string) (*NamespacedCertificate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NamespacedCertificate), err
}

func (s *namespacedCertificateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedCertificateClient) AddHandler(name string, sync NamespacedCertificateHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *namespacedCertificateClient) AddLifecycle(name string, lifecycle NamespacedCertificateLifecycle) {
	sync := NewNamespacedCertificateLifecycleAdapter(name, s, lifecycle)
	s.AddHandler(name, sync)
}
