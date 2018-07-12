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
	NotifierGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Notifier",
	}
	NotifierResource = metav1.APIResource{
		Name:         "notifiers",
		SingularName: "notifier",
		Namespaced:   true,

		Kind: NotifierGroupVersionKind.Kind,
	}
)

type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Notifier
}

type NotifierHandlerFunc func(key string, obj *Notifier) error

type NotifierLister interface {
	List(namespace string, selector labels.Selector) (ret []*Notifier, err error)
	Get(namespace, name string) (*Notifier, error)
}

type NotifierController interface {
	Informer() cache.SharedIndexInformer
	Lister() NotifierLister
	AddHandler(name string, handler NotifierHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler NotifierHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NotifierInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Notifier) (*Notifier, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Notifier, error)
	Get(name string, opts metav1.GetOptions) (*Notifier, error)
	Update(*Notifier) (*Notifier, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NotifierList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NotifierController
	AddHandler(name string, sync NotifierHandlerFunc)
	AddLifecycle(name string, lifecycle NotifierLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync NotifierHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle NotifierLifecycle)
}

type notifierLister struct {
	controller *notifierController
}

func (l *notifierLister) List(namespace string, selector labels.Selector) (ret []*Notifier, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Notifier))
	})
	return
}

func (l *notifierLister) Get(namespace, name string) (*Notifier, error) {
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
			Group:    NotifierGroupVersionKind.Group,
			Resource: "notifier",
		}, key)
	}
	return obj.(*Notifier), nil
}

type notifierController struct {
	controller.GenericController
}

func (c *notifierController) Lister() NotifierLister {
	return &notifierLister{
		controller: c,
	}
}

func (c *notifierController) AddHandler(name string, handler NotifierHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Notifier))
	})
}

func (c *notifierController) AddClusterScopedHandler(name, cluster string, handler NotifierHandlerFunc) {
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

		return handler(key, obj.(*Notifier))
	})
}

type notifierFactory struct {
}

func (c notifierFactory) Object() runtime.Object {
	return &Notifier{}
}

func (c notifierFactory) List() runtime.Object {
	return &NotifierList{}
}

func (s *notifierClient) Controller() NotifierController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.notifierControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NotifierGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &notifierController{
		GenericController: genericController,
	}

	s.client.notifierControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type notifierClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NotifierController
}

func (s *notifierClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *notifierClient) Create(o *Notifier) (*Notifier, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Notifier), err
}

func (s *notifierClient) Get(name string, opts metav1.GetOptions) (*Notifier, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Notifier), err
}

func (s *notifierClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Notifier, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Notifier), err
}

func (s *notifierClient) Update(o *Notifier) (*Notifier, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Notifier), err
}

func (s *notifierClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *notifierClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *notifierClient) List(opts metav1.ListOptions) (*NotifierList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NotifierList), err
}

func (s *notifierClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *notifierClient) Patch(o *Notifier, data []byte, subresources ...string) (*Notifier, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Notifier), err
}

func (s *notifierClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *notifierClient) AddHandler(name string, sync NotifierHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *notifierClient) AddLifecycle(name string, lifecycle NotifierLifecycle) {
	sync := NewNotifierLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *notifierClient) AddClusterScopedHandler(name, clusterName string, sync NotifierHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *notifierClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle NotifierLifecycle) {
	sync := NewNotifierLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
