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
	FreeIpaTestAndApplyInputGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "FreeIpaTestAndApplyInput",
	}
	FreeIpaTestAndApplyInputResource = metav1.APIResource{
		Name:         "freeipatestandapplyinputs",
		SingularName: "freeipatestandapplyinput",
		Namespaced:   false,
		Kind:         FreeIpaTestAndApplyInputGroupVersionKind.Kind,
	}
)

type FreeIpaTestAndApplyInputList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FreeIpaTestAndApplyInput
}

type FreeIpaTestAndApplyInputHandlerFunc func(key string, obj *FreeIpaTestAndApplyInput) error

type FreeIpaTestAndApplyInputLister interface {
	List(namespace string, selector labels.Selector) (ret []*FreeIpaTestAndApplyInput, err error)
	Get(namespace, name string) (*FreeIpaTestAndApplyInput, error)
}

type FreeIpaTestAndApplyInputController interface {
	Informer() cache.SharedIndexInformer
	Lister() FreeIpaTestAndApplyInputLister
	AddHandler(name string, handler FreeIpaTestAndApplyInputHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler FreeIpaTestAndApplyInputHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type FreeIpaTestAndApplyInputInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*FreeIpaTestAndApplyInput, error)
	Get(name string, opts metav1.GetOptions) (*FreeIpaTestAndApplyInput, error)
	Update(*FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*FreeIpaTestAndApplyInputList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() FreeIpaTestAndApplyInputController
	AddHandler(name string, sync FreeIpaTestAndApplyInputHandlerFunc)
	AddLifecycle(name string, lifecycle FreeIpaTestAndApplyInputLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync FreeIpaTestAndApplyInputHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle FreeIpaTestAndApplyInputLifecycle)
}

type freeIpaTestAndApplyInputLister struct {
	controller *freeIpaTestAndApplyInputController
}

func (l *freeIpaTestAndApplyInputLister) List(namespace string, selector labels.Selector) (ret []*FreeIpaTestAndApplyInput, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*FreeIpaTestAndApplyInput))
	})
	return
}

func (l *freeIpaTestAndApplyInputLister) Get(namespace, name string) (*FreeIpaTestAndApplyInput, error) {
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
			Group:    FreeIpaTestAndApplyInputGroupVersionKind.Group,
			Resource: "freeIpaTestAndApplyInput",
		}, name)
	}
	return obj.(*FreeIpaTestAndApplyInput), nil
}

type freeIpaTestAndApplyInputController struct {
	controller.GenericController
}

func (c *freeIpaTestAndApplyInputController) Lister() FreeIpaTestAndApplyInputLister {
	return &freeIpaTestAndApplyInputLister{
		controller: c,
	}
}

func (c *freeIpaTestAndApplyInputController) AddHandler(name string, handler FreeIpaTestAndApplyInputHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*FreeIpaTestAndApplyInput))
	})
}

func (c *freeIpaTestAndApplyInputController) AddClusterScopedHandler(name, cluster string, handler FreeIpaTestAndApplyInputHandlerFunc) {
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

		return handler(key, obj.(*FreeIpaTestAndApplyInput))
	})
}

type freeIpaTestAndApplyInputFactory struct {
}

func (c freeIpaTestAndApplyInputFactory) Object() runtime.Object {
	return &FreeIpaTestAndApplyInput{}
}

func (c freeIpaTestAndApplyInputFactory) List() runtime.Object {
	return &FreeIpaTestAndApplyInputList{}
}

func (s *freeIpaTestAndApplyInputClient) Controller() FreeIpaTestAndApplyInputController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.freeIpaTestAndApplyInputControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(FreeIpaTestAndApplyInputGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &freeIpaTestAndApplyInputController{
		GenericController: genericController,
	}

	s.client.freeIpaTestAndApplyInputControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type freeIpaTestAndApplyInputClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   FreeIpaTestAndApplyInputController
}

func (s *freeIpaTestAndApplyInputClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *freeIpaTestAndApplyInputClient) Create(o *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*FreeIpaTestAndApplyInput), err
}

func (s *freeIpaTestAndApplyInputClient) Get(name string, opts metav1.GetOptions) (*FreeIpaTestAndApplyInput, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*FreeIpaTestAndApplyInput), err
}

func (s *freeIpaTestAndApplyInputClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*FreeIpaTestAndApplyInput, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*FreeIpaTestAndApplyInput), err
}

func (s *freeIpaTestAndApplyInputClient) Update(o *FreeIpaTestAndApplyInput) (*FreeIpaTestAndApplyInput, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*FreeIpaTestAndApplyInput), err
}

func (s *freeIpaTestAndApplyInputClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *freeIpaTestAndApplyInputClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *freeIpaTestAndApplyInputClient) List(opts metav1.ListOptions) (*FreeIpaTestAndApplyInputList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*FreeIpaTestAndApplyInputList), err
}

func (s *freeIpaTestAndApplyInputClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *freeIpaTestAndApplyInputClient) Patch(o *FreeIpaTestAndApplyInput, data []byte, subresources ...string) (*FreeIpaTestAndApplyInput, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*FreeIpaTestAndApplyInput), err
}

func (s *freeIpaTestAndApplyInputClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *freeIpaTestAndApplyInputClient) AddHandler(name string, sync FreeIpaTestAndApplyInputHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *freeIpaTestAndApplyInputClient) AddLifecycle(name string, lifecycle FreeIpaTestAndApplyInputLifecycle) {
	sync := NewFreeIpaTestAndApplyInputLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *freeIpaTestAndApplyInputClient) AddClusterScopedHandler(name, clusterName string, sync FreeIpaTestAndApplyInputHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *freeIpaTestAndApplyInputClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle FreeIpaTestAndApplyInputLifecycle) {
	sync := NewFreeIpaTestAndApplyInputLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
