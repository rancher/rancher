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
	UserAttributeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "UserAttribute",
	}
	UserAttributeResource = metav1.APIResource{
		Name:         "userattributes",
		SingularName: "userattribute",
		Namespaced:   false,
		Kind:         UserAttributeGroupVersionKind.Kind,
	}

	UserAttributeGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "userattributes",
	}
)

func init() {
	resource.Put(UserAttributeGroupVersionResource)
}

func NewUserAttribute(namespace, name string, obj UserAttribute) *UserAttribute {
	obj.APIVersion, obj.Kind = UserAttributeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type UserAttributeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserAttribute `json:"items"`
}

type UserAttributeHandlerFunc func(key string, obj *UserAttribute) (runtime.Object, error)

type UserAttributeChangeHandlerFunc func(obj *UserAttribute) (runtime.Object, error)

type UserAttributeLister interface {
	List(namespace string, selector labels.Selector) (ret []*UserAttribute, err error)
	Get(namespace, name string) (*UserAttribute, error)
}

type UserAttributeController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() UserAttributeLister
	AddHandler(ctx context.Context, name string, handler UserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserAttributeHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler UserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler UserAttributeHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type UserAttributeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*UserAttribute) (*UserAttribute, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*UserAttribute, error)
	Get(name string, opts metav1.GetOptions) (*UserAttribute, error)
	Update(*UserAttribute) (*UserAttribute, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*UserAttributeList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*UserAttributeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() UserAttributeController
	AddHandler(ctx context.Context, name string, sync UserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserAttributeHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle UserAttributeLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle UserAttributeLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync UserAttributeHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserAttributeLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle UserAttributeLifecycle)
}

type userAttributeLister struct {
	controller *userAttributeController
}

func (l *userAttributeLister) List(namespace string, selector labels.Selector) (ret []*UserAttribute, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*UserAttribute))
	})
	return
}

func (l *userAttributeLister) Get(namespace, name string) (*UserAttribute, error) {
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
			Group:    UserAttributeGroupVersionKind.Group,
			Resource: "userAttribute",
		}, key)
	}
	return obj.(*UserAttribute), nil
}

type userAttributeController struct {
	controller.GenericController
}

func (c *userAttributeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *userAttributeController) Lister() UserAttributeLister {
	return &userAttributeLister{
		controller: c,
	}
}

func (c *userAttributeController) AddHandler(ctx context.Context, name string, handler UserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userAttributeController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler UserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userAttributeController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler UserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userAttributeController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler UserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type userAttributeFactory struct {
}

func (c userAttributeFactory) Object() runtime.Object {
	return &UserAttribute{}
}

func (c userAttributeFactory) List() runtime.Object {
	return &UserAttributeList{}
}

func (s *userAttributeClient) Controller() UserAttributeController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.userAttributeControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(UserAttributeGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &userAttributeController{
		GenericController: genericController,
	}

	s.client.userAttributeControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type userAttributeClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   UserAttributeController
}

func (s *userAttributeClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *userAttributeClient) Create(o *UserAttribute) (*UserAttribute, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) Get(name string, opts metav1.GetOptions) (*UserAttribute, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*UserAttribute, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) Update(o *UserAttribute) (*UserAttribute, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *userAttributeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *userAttributeClient) List(opts metav1.ListOptions) (*UserAttributeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*UserAttributeList), err
}

func (s *userAttributeClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*UserAttributeList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*UserAttributeList), err
}

func (s *userAttributeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *userAttributeClient) Patch(o *UserAttribute, patchType types.PatchType, data []byte, subresources ...string) (*UserAttribute, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *userAttributeClient) AddHandler(ctx context.Context, name string, sync UserAttributeHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userAttributeClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserAttributeHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *userAttributeClient) AddLifecycle(ctx context.Context, name string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userAttributeClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *userAttributeClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userAttributeClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync UserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *userAttributeClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userAttributeClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
