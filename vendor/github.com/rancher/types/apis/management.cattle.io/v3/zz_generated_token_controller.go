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
	TokenGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Token",
	}
	TokenResource = metav1.APIResource{
		Name:         "tokens",
		SingularName: "token",
		Namespaced:   false,
		Kind:         TokenGroupVersionKind.Kind,
	}
)

type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Token
}

type TokenHandlerFunc func(key string, obj *Token) error

type TokenLister interface {
	List(namespace string, selector labels.Selector) (ret []*Token, err error)
	Get(namespace, name string) (*Token, error)
}

type TokenController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TokenLister
	AddHandler(name string, handler TokenHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler TokenHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TokenInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Token) (*Token, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Token, error)
	Get(name string, opts metav1.GetOptions) (*Token, error)
	Update(*Token) (*Token, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TokenList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TokenController
	AddHandler(name string, sync TokenHandlerFunc)
	AddLifecycle(name string, lifecycle TokenLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync TokenHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle TokenLifecycle)
}

type tokenLister struct {
	controller *tokenController
}

func (l *tokenLister) List(namespace string, selector labels.Selector) (ret []*Token, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Token))
	})
	return
}

func (l *tokenLister) Get(namespace, name string) (*Token, error) {
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
			Group:    TokenGroupVersionKind.Group,
			Resource: "token",
		}, key)
	}
	return obj.(*Token), nil
}

type tokenController struct {
	controller.GenericController
}

func (c *tokenController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *tokenController) Lister() TokenLister {
	return &tokenLister{
		controller: c,
	}
}

func (c *tokenController) AddHandler(name string, handler TokenHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Token))
	})
}

func (c *tokenController) AddClusterScopedHandler(name, cluster string, handler TokenHandlerFunc) {
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

		return handler(key, obj.(*Token))
	})
}

type tokenFactory struct {
}

func (c tokenFactory) Object() runtime.Object {
	return &Token{}
}

func (c tokenFactory) List() runtime.Object {
	return &TokenList{}
}

func (s *tokenClient) Controller() TokenController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.tokenControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TokenGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &tokenController{
		GenericController: genericController,
	}

	s.client.tokenControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type tokenClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TokenController
}

func (s *tokenClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *tokenClient) Create(o *Token) (*Token, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Token), err
}

func (s *tokenClient) Get(name string, opts metav1.GetOptions) (*Token, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Token), err
}

func (s *tokenClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Token, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Token), err
}

func (s *tokenClient) Update(o *Token) (*Token, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Token), err
}

func (s *tokenClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *tokenClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *tokenClient) List(opts metav1.ListOptions) (*TokenList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TokenList), err
}

func (s *tokenClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *tokenClient) Patch(o *Token, data []byte, subresources ...string) (*Token, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Token), err
}

func (s *tokenClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *tokenClient) AddHandler(name string, sync TokenHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *tokenClient) AddLifecycle(name string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *tokenClient) AddClusterScopedHandler(name, clusterName string, sync TokenHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *tokenClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle TokenLifecycle) {
	sync := NewTokenLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
