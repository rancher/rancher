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
	OpenLdapTestAndApplyInputGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "OpenLdapTestAndApplyInput",
	}
	OpenLdapTestAndApplyInputResource = metav1.APIResource{
		Name:         "openldaptestandapplyinputs",
		SingularName: "openldaptestandapplyinput",
		Namespaced:   false,
		Kind:         OpenLdapTestAndApplyInputGroupVersionKind.Kind,
	}
)

type OpenLdapTestAndApplyInputList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenLdapTestAndApplyInput
}

type OpenLdapTestAndApplyInputHandlerFunc func(key string, obj *OpenLdapTestAndApplyInput) error

type OpenLdapTestAndApplyInputLister interface {
	List(namespace string, selector labels.Selector) (ret []*OpenLdapTestAndApplyInput, err error)
	Get(namespace, name string) (*OpenLdapTestAndApplyInput, error)
}

type OpenLdapTestAndApplyInputController interface {
	Informer() cache.SharedIndexInformer
	Lister() OpenLdapTestAndApplyInputLister
	AddHandler(name string, handler OpenLdapTestAndApplyInputHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler OpenLdapTestAndApplyInputHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type OpenLdapTestAndApplyInputInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*OpenLdapTestAndApplyInput, error)
	Get(name string, opts metav1.GetOptions) (*OpenLdapTestAndApplyInput, error)
	Update(*OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*OpenLdapTestAndApplyInputList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() OpenLdapTestAndApplyInputController
	AddHandler(name string, sync OpenLdapTestAndApplyInputHandlerFunc)
	AddLifecycle(name string, lifecycle OpenLdapTestAndApplyInputLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync OpenLdapTestAndApplyInputHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle OpenLdapTestAndApplyInputLifecycle)
}

type openLdapTestAndApplyInputLister struct {
	controller *openLdapTestAndApplyInputController
}

func (l *openLdapTestAndApplyInputLister) List(namespace string, selector labels.Selector) (ret []*OpenLdapTestAndApplyInput, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*OpenLdapTestAndApplyInput))
	})
	return
}

func (l *openLdapTestAndApplyInputLister) Get(namespace, name string) (*OpenLdapTestAndApplyInput, error) {
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
			Group:    OpenLdapTestAndApplyInputGroupVersionKind.Group,
			Resource: "openLdapTestAndApplyInput",
		}, name)
	}
	return obj.(*OpenLdapTestAndApplyInput), nil
}

type openLdapTestAndApplyInputController struct {
	controller.GenericController
}

func (c *openLdapTestAndApplyInputController) Lister() OpenLdapTestAndApplyInputLister {
	return &openLdapTestAndApplyInputLister{
		controller: c,
	}
}

func (c *openLdapTestAndApplyInputController) AddHandler(name string, handler OpenLdapTestAndApplyInputHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*OpenLdapTestAndApplyInput))
	})
}

func (c *openLdapTestAndApplyInputController) AddClusterScopedHandler(name, cluster string, handler OpenLdapTestAndApplyInputHandlerFunc) {
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

		return handler(key, obj.(*OpenLdapTestAndApplyInput))
	})
}

type openLdapTestAndApplyInputFactory struct {
}

func (c openLdapTestAndApplyInputFactory) Object() runtime.Object {
	return &OpenLdapTestAndApplyInput{}
}

func (c openLdapTestAndApplyInputFactory) List() runtime.Object {
	return &OpenLdapTestAndApplyInputList{}
}

func (s *openLdapTestAndApplyInputClient) Controller() OpenLdapTestAndApplyInputController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.openLdapTestAndApplyInputControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(OpenLdapTestAndApplyInputGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &openLdapTestAndApplyInputController{
		GenericController: genericController,
	}

	s.client.openLdapTestAndApplyInputControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type openLdapTestAndApplyInputClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   OpenLdapTestAndApplyInputController
}

func (s *openLdapTestAndApplyInputClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *openLdapTestAndApplyInputClient) Create(o *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*OpenLdapTestAndApplyInput), err
}

func (s *openLdapTestAndApplyInputClient) Get(name string, opts metav1.GetOptions) (*OpenLdapTestAndApplyInput, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*OpenLdapTestAndApplyInput), err
}

func (s *openLdapTestAndApplyInputClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*OpenLdapTestAndApplyInput, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*OpenLdapTestAndApplyInput), err
}

func (s *openLdapTestAndApplyInputClient) Update(o *OpenLdapTestAndApplyInput) (*OpenLdapTestAndApplyInput, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*OpenLdapTestAndApplyInput), err
}

func (s *openLdapTestAndApplyInputClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *openLdapTestAndApplyInputClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *openLdapTestAndApplyInputClient) List(opts metav1.ListOptions) (*OpenLdapTestAndApplyInputList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*OpenLdapTestAndApplyInputList), err
}

func (s *openLdapTestAndApplyInputClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *openLdapTestAndApplyInputClient) Patch(o *OpenLdapTestAndApplyInput, data []byte, subresources ...string) (*OpenLdapTestAndApplyInput, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*OpenLdapTestAndApplyInput), err
}

func (s *openLdapTestAndApplyInputClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *openLdapTestAndApplyInputClient) AddHandler(name string, sync OpenLdapTestAndApplyInputHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *openLdapTestAndApplyInputClient) AddLifecycle(name string, lifecycle OpenLdapTestAndApplyInputLifecycle) {
	sync := NewOpenLdapTestAndApplyInputLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *openLdapTestAndApplyInputClient) AddClusterScopedHandler(name, clusterName string, sync OpenLdapTestAndApplyInputHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *openLdapTestAndApplyInputClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle OpenLdapTestAndApplyInputLifecycle) {
	sync := NewOpenLdapTestAndApplyInputLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
