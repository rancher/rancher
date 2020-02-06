package v3

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
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
	SSHAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SSHAuth",
	}
	SSHAuthResource = metav1.APIResource{
		Name:         "sshauths",
		SingularName: "sshauth",
		Namespaced:   true,

		Kind: SSHAuthGroupVersionKind.Kind,
	}

	SSHAuthGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "sshauths",
	}
)

func init() {
	resource.Put(SSHAuthGroupVersionResource)
}

func NewSSHAuth(namespace, name string, obj SSHAuth) *SSHAuth {
	obj.APIVersion, obj.Kind = SSHAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SSHAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHAuth `json:"items"`
}

type SSHAuthHandlerFunc func(key string, obj *SSHAuth) (runtime.Object, error)

type SSHAuthChangeHandlerFunc func(obj *SSHAuth) (runtime.Object, error)

type SSHAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*SSHAuth, err error)
	Get(namespace, name string) (*SSHAuth, error)
}

type SSHAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SSHAuthLister
	AddHandler(ctx context.Context, name string, handler SSHAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SSHAuthHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SSHAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler SSHAuthHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SSHAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SSHAuth) (*SSHAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SSHAuth, error)
	Get(name string, opts metav1.GetOptions) (*SSHAuth, error)
	Update(*SSHAuth) (*SSHAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SSHAuthList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*SSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SSHAuthController
	AddHandler(ctx context.Context, name string, sync SSHAuthHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SSHAuthHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SSHAuthLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SSHAuthLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SSHAuthHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SSHAuthHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SSHAuthLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SSHAuthLifecycle)
}

type sshAuthLister struct {
	controller *sshAuthController
}

func (l *sshAuthLister) List(namespace string, selector labels.Selector) (ret []*SSHAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SSHAuth))
	})
	return
}

func (l *sshAuthLister) Get(namespace, name string) (*SSHAuth, error) {
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
			Group:    SSHAuthGroupVersionKind.Group,
			Resource: "sshAuth",
		}, key)
	}
	return obj.(*SSHAuth), nil
}

type sshAuthController struct {
	controller.GenericController
}

func (c *sshAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sshAuthController) Lister() SSHAuthLister {
	return &sshAuthLister{
		controller: c,
	}
}

func (c *sshAuthController) AddHandler(ctx context.Context, name string, handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sshAuthController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sshAuthController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sshAuthController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sshAuthFactory struct {
}

func (c sshAuthFactory) Object() runtime.Object {
	return &SSHAuth{}
}

func (c sshAuthFactory) List() runtime.Object {
	return &SSHAuthList{}
}

func (s *sshAuthClient) Controller() SSHAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sshAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SSHAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sshAuthController{
		GenericController: genericController,
	}

	s.client.sshAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sshAuthClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SSHAuthController
}

func (s *sshAuthClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sshAuthClient) Create(o *SSHAuth) (*SSHAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Get(name string, opts metav1.GetOptions) (*SSHAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SSHAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Update(o *SSHAuth) (*SSHAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sshAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sshAuthClient) List(opts metav1.ListOptions) (*SSHAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SSHAuthList), err
}

func (s *sshAuthClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*SSHAuthList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*SSHAuthList), err
}

func (s *sshAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sshAuthClient) Patch(o *SSHAuth, patchType types.PatchType, data []byte, subresources ...string) (*SSHAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sshAuthClient) AddHandler(ctx context.Context, name string, sync SSHAuthHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sshAuthClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync SSHAuthHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sshAuthClient) AddLifecycle(ctx context.Context, name string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sshAuthClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *sshAuthClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sshAuthClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync SSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *sshAuthClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sshAuthClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
