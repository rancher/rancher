package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	EtcdBackupGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "EtcdBackup",
	}
	EtcdBackupResource = metav1.APIResource{
		Name:         "etcdbackups",
		SingularName: "etcdbackup",
		Namespaced:   true,

		Kind: EtcdBackupGroupVersionKind.Kind,
	}

	EtcdBackupGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "etcdbackups",
	}
)

func init() {
	resource.Put(EtcdBackupGroupVersionResource)
}

// Deprecated: use v3.EtcdBackup instead
type EtcdBackup = v3.EtcdBackup

func NewEtcdBackup(namespace, name string, obj v3.EtcdBackup) *v3.EtcdBackup {
	obj.APIVersion, obj.Kind = EtcdBackupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type EtcdBackupHandlerFunc func(key string, obj *v3.EtcdBackup) (runtime.Object, error)

type EtcdBackupChangeHandlerFunc func(obj *v3.EtcdBackup) (runtime.Object, error)

type EtcdBackupLister interface {
	List(namespace string, selector labels.Selector) (ret []*v3.EtcdBackup, err error)
	Get(namespace, name string) (*v3.EtcdBackup, error)
}

type EtcdBackupController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() EtcdBackupLister
	AddHandler(ctx context.Context, name string, handler EtcdBackupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EtcdBackupHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler EtcdBackupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler EtcdBackupHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type EtcdBackupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v3.EtcdBackup) (*v3.EtcdBackup, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.EtcdBackup, error)
	Get(name string, opts metav1.GetOptions) (*v3.EtcdBackup, error)
	Update(*v3.EtcdBackup) (*v3.EtcdBackup, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v3.EtcdBackupList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.EtcdBackupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() EtcdBackupController
	AddHandler(ctx context.Context, name string, sync EtcdBackupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EtcdBackupHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle EtcdBackupLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EtcdBackupLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EtcdBackupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EtcdBackupHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EtcdBackupLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EtcdBackupLifecycle)
}

type etcdBackupLister struct {
	ns         string
	controller *etcdBackupController
}

func (l *etcdBackupLister) List(namespace string, selector labels.Selector) (ret []*v3.EtcdBackup, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v3.EtcdBackup))
	})
	return
}

func (l *etcdBackupLister) Get(namespace, name string) (*v3.EtcdBackup, error) {
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
			Group:    EtcdBackupGroupVersionKind.Group,
			Resource: EtcdBackupGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v3.EtcdBackup), nil
}

type etcdBackupController struct {
	ns string
	controller.GenericController
}

func (c *etcdBackupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *etcdBackupController) Lister() EtcdBackupLister {
	return &etcdBackupLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *etcdBackupController) AddHandler(ctx context.Context, name string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EtcdBackup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *etcdBackupController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EtcdBackup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *etcdBackupController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EtcdBackup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *etcdBackupController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v3.EtcdBackup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type etcdBackupFactory struct {
}

func (c etcdBackupFactory) Object() runtime.Object {
	return &v3.EtcdBackup{}
}

func (c etcdBackupFactory) List() runtime.Object {
	return &v3.EtcdBackupList{}
}

func (s *etcdBackupClient) Controller() EtcdBackupController {
	genericController := controller.NewGenericController(s.ns, EtcdBackupGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(EtcdBackupGroupVersionResource, EtcdBackupGroupVersionKind.Kind, true))

	return &etcdBackupController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type etcdBackupClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   EtcdBackupController
}

func (s *etcdBackupClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *etcdBackupClient) Create(o *v3.EtcdBackup) (*v3.EtcdBackup, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v3.EtcdBackup), err
}

func (s *etcdBackupClient) Get(name string, opts metav1.GetOptions) (*v3.EtcdBackup, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v3.EtcdBackup), err
}

func (s *etcdBackupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v3.EtcdBackup, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v3.EtcdBackup), err
}

func (s *etcdBackupClient) Update(o *v3.EtcdBackup) (*v3.EtcdBackup, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v3.EtcdBackup), err
}

func (s *etcdBackupClient) UpdateStatus(o *v3.EtcdBackup) (*v3.EtcdBackup, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v3.EtcdBackup), err
}

func (s *etcdBackupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *etcdBackupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *etcdBackupClient) List(opts metav1.ListOptions) (*v3.EtcdBackupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v3.EtcdBackupList), err
}

func (s *etcdBackupClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v3.EtcdBackupList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v3.EtcdBackupList), err
}

func (s *etcdBackupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *etcdBackupClient) Patch(o *v3.EtcdBackup, patchType types.PatchType, data []byte, subresources ...string) (*v3.EtcdBackup, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v3.EtcdBackup), err
}

func (s *etcdBackupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *etcdBackupClient) AddHandler(ctx context.Context, name string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *etcdBackupClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *etcdBackupClient) AddLifecycle(ctx context.Context, name string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *etcdBackupClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *etcdBackupClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *etcdBackupClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *etcdBackupClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *etcdBackupClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
