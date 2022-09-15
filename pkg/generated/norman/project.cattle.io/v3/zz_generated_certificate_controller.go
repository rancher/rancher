package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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

	CertificateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "certificates",
	}
)

func init() {
	resource.Put(CertificateGroupVersionResource)
}

// Deprecated: use v3.Certificate instead
type Certificate = v3.Certificate

func NewCertificate(namespace, name string, obj v3.Certificate) *v3.Certificate {
	obj.APIVersion, obj.Kind = CertificateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CertificateHandlerFunc func(key string, obj *v3.Certificate) (runtime.Object, error)

type CertificateChangeHandlerFunc func(obj *v3.Certificate) (runtime.Object, error)

type CertificateLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.Certificate, err error)
	Get(namespace, name string) (*v3.Certificate, error)
}

type CertificateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CertificateLister
	AddHandler(ctx context.Context, name string, handler CertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CertificateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CertificateHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type CertificateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.Certificate) (*v3.Certificate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Certificate, error)
	Get(name string, opts metav1.GetOptions) (*v3.Certificate, error)
	Update(*v3.Certificate) (*v3.Certificate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.CertificateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CertificateController
	AddHandler(ctx context.Context, name string, sync CertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CertificateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CertificateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CertificateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CertificateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CertificateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CertificateLifecycle)
}

type certificateLister struct {
	ns         string
	controller *certificateController
}

func (l *certificateLister) List(namespace string, selector labels.Selector) (ret []*v3.Certificate, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.Certificate))
	})
	return
}

func (l *certificateLister) Get(namespace, name string) (*v3.Certificate, error) {
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
			Resource: CertificateGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.Certificate), nil
}

type certificateController struct {
	ns string
	controller.GenericController
}

func (c *certificateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *certificateController) Lister() CertificateLister {
	return &certificateLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *certificateController) AddHandler(ctx context.Context, name string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Certificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *certificateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Certificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *certificateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Certificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *certificateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.Certificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type certificateFactory struct {
}

func (c certificateFactory) Object() runtime.Object {
	return &v3.Certificate{}
}

func (c certificateFactory) List() runtime.Object {
	return &v3.CertificateList{}
}

func (s *certificateClient) Controller() CertificateController {
	genericController := controller.NewGenericController(s.ns, CertificateGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(CertificateGroupVersionResource, CertificateGroupVersionKind.Kind, true))

	return &certificateController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *certificateClient) Create(o *v3.Certificate) (*v3.Certificate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.Certificate), err
}

func (s *certificateClient) Get(name string, opts metav1.GetOptions) (*v3.Certificate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.Certificate), err
}

func (s *certificateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.Certificate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.Certificate), err
}

func (s *certificateClient) Update(o *v3.Certificate) (*v3.Certificate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.Certificate), err
}

func (s *certificateClient) UpdateStatus(o *v3.Certificate) (*v3.Certificate, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.Certificate), err
}

func (s *certificateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *certificateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *certificateClient) List(opts metav1.ListOptions) (*v3.CertificateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.CertificateList), err
}

func (s *certificateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.CertificateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.CertificateList), err
}

func (s *certificateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *certificateClient) Patch(o *v3.Certificate, patchType types.PatchType, data []byte, subresources ...string) (*v3.Certificate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.Certificate), err
}

func (s *certificateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *certificateClient) AddHandler(ctx context.Context, name string, sync CertificateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *certificateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CertificateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *certificateClient) AddLifecycle(ctx context.Context, name string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *certificateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *certificateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CertificateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CertificateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
