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
	GlobalRoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "GlobalRoleBinding",
	}
	GlobalRoleBindingResource = metav1.APIResource{
		Name:         "globalrolebindings",
		SingularName: "globalrolebinding",
		Namespaced:   false,
		Kind:         GlobalRoleBindingGroupVersionKind.Kind,
	}
)

type GlobalRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalRoleBinding
}

type GlobalRoleBindingHandlerFunc func(key string, obj *GlobalRoleBinding) error

type GlobalRoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*GlobalRoleBinding, err error)
	Get(namespace, name string) (*GlobalRoleBinding, error)
}

type GlobalRoleBindingController interface {
	Informer() cache.SharedIndexInformer
	Lister() GlobalRoleBindingLister
	AddHandler(handler GlobalRoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GlobalRoleBindingInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*GlobalRoleBinding) (*GlobalRoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*GlobalRoleBinding, error)
	Update(*GlobalRoleBinding) (*GlobalRoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GlobalRoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GlobalRoleBindingController
	AddSyncHandler(sync GlobalRoleBindingHandlerFunc)
	AddLifecycle(name string, lifecycle GlobalRoleBindingLifecycle)
}

type globalRoleBindingLister struct {
	controller *globalRoleBindingController
}

func (l *globalRoleBindingLister) List(namespace string, selector labels.Selector) (ret []*GlobalRoleBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GlobalRoleBinding))
	})
	return
}

func (l *globalRoleBindingLister) Get(namespace, name string) (*GlobalRoleBinding, error) {
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
			Group:    GlobalRoleBindingGroupVersionKind.Group,
			Resource: "globalRoleBinding",
		}, name)
	}
	return obj.(*GlobalRoleBinding), nil
}

type globalRoleBindingController struct {
	controller.GenericController
}

func (c *globalRoleBindingController) Lister() GlobalRoleBindingLister {
	return &globalRoleBindingLister{
		controller: c,
	}
}

func (c *globalRoleBindingController) AddHandler(handler GlobalRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*GlobalRoleBinding))
	})
}

type globalRoleBindingFactory struct {
}

func (c globalRoleBindingFactory) Object() runtime.Object {
	return &GlobalRoleBinding{}
}

func (c globalRoleBindingFactory) List() runtime.Object {
	return &GlobalRoleBindingList{}
}

func (s *globalRoleBindingClient) Controller() GlobalRoleBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.globalRoleBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GlobalRoleBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &globalRoleBindingController{
		GenericController: genericController,
	}

	s.client.globalRoleBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type globalRoleBindingClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   GlobalRoleBindingController
}

func (s *globalRoleBindingClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *globalRoleBindingClient) Create(o *GlobalRoleBinding) (*GlobalRoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) Get(name string, opts metav1.GetOptions) (*GlobalRoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) Update(o *GlobalRoleBinding) (*GlobalRoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *globalRoleBindingClient) List(opts metav1.ListOptions) (*GlobalRoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GlobalRoleBindingList), err
}

func (s *globalRoleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *globalRoleBindingClient) Patch(o *GlobalRoleBinding, data []byte, subresources ...string) (*GlobalRoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*GlobalRoleBinding), err
}

func (s *globalRoleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *globalRoleBindingClient) AddSyncHandler(sync GlobalRoleBindingHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *globalRoleBindingClient) AddLifecycle(name string, lifecycle GlobalRoleBindingLifecycle) {
	sync := NewGlobalRoleBindingLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
