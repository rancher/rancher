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
	CertificateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Certificate",
	}
	CertificateResource = metav1.APIResource{
		Name:         "certificates",
		SingularName: "certificate",
		Namespaced:   true,

		Kind: CertificateGroupVersionKind.Kind,
	}
)

type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate
}

type CertificateHandlerFunc func(key string, obj *Certificate) error

type CertificateLister interface {
	List(namespace string, selector labels.Selector) (ret []*Certificate, err error)
	Get(namespace, name string) (*Certificate, error)
}

type CertificateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CertificateLister
	AddHandler(name string, handler CertificateHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler CertificateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CertificateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Certificate) (*Certificate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Certificate, error)
	Get(name string, opts metav1.GetOptions) (*Certificate, error)
	Update(*Certificate) (*Certificate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CertificateController
	AddHandler(name string, sync CertificateHandlerFunc)
	AddLifecycle(name string, lifecycle CertificateLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync CertificateHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle CertificateLifecycle)
}

type certificateLister struct {
	controller *certificateController
}

func (l *certificateLister) List(namespace string, selector labels.Selector) (ret []*Certificate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Certificate))
	})
	return
}

func (l *certificateLister) Get(namespace, name string) (*Certificate, error) {
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
			Group:    CertificateGroupVersionKind.Group,
			Resource: "certificate",
		}, key)
	}
	return obj.(*Certificate), nil
}

type certificateController struct {
	controller.GenericController
}

func (c *certificateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *certificateController) Lister() CertificateLister {
	return &certificateLister{
		controller: c,
	}
}

func (c *certificateController) AddHandler(name string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Certificate))
	})
}

func (c *certificateController) AddClusterScopedHandler(name, cluster string, handler CertificateHandlerFunc) {
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

		return handler(key, obj.(*Certificate))
	})
}

type certificateFactory struct {
}

func (c certificateFactory) Object() runtime.Object {
	return &Certificate{}
}

func (c certificateFactory) List() runtime.Object {
	return &CertificateList{}
}

func (s *certificateClient) Controller() CertificateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.certificateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CertificateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &certificateController{
		GenericController: genericController,
	}

	s.client.certificateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type certificateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CertificateController
}

func (s *certificateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *certificateClient) Create(o *Certificate) (*Certificate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Certificate), err
}

func (s *certificateClient) Get(name string, opts metav1.GetOptions) (*Certificate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Certificate), err
}

func (s *certificateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Certificate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Certificate), err
}

func (s *certificateClient) Update(o *Certificate) (*Certificate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Certificate), err
}

func (s *certificateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *certificateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *certificateClient) List(opts metav1.ListOptions) (*CertificateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CertificateList), err
}

func (s *certificateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *certificateClient) Patch(o *Certificate, data []byte, subresources ...string) (*Certificate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Certificate), err
}

func (s *certificateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *certificateClient) AddHandler(name string, sync CertificateHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *certificateClient) AddLifecycle(name string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *certificateClient) AddClusterScopedHandler(name, clusterName string, sync CertificateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
