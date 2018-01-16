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
	MachineDriverGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "MachineDriver",
	}
	MachineDriverResource = metav1.APIResource{
		Name:         "machinedrivers",
		SingularName: "machinedriver",
		Namespaced:   false,
		Kind:         MachineDriverGroupVersionKind.Kind,
	}
)

type MachineDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineDriver
}

type MachineDriverHandlerFunc func(key string, obj *MachineDriver) error

type MachineDriverLister interface {
	List(namespace string, selector labels.Selector) (ret []*MachineDriver, err error)
	Get(namespace, name string) (*MachineDriver, error)
}

type MachineDriverController interface {
	Informer() cache.SharedIndexInformer
	Lister() MachineDriverLister
	AddHandler(name string, handler MachineDriverHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type MachineDriverInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*MachineDriver) (*MachineDriver, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*MachineDriver, error)
	Get(name string, opts metav1.GetOptions) (*MachineDriver, error)
	Update(*MachineDriver) (*MachineDriver, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*MachineDriverList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MachineDriverController
	AddHandler(name string, sync MachineDriverHandlerFunc)
	AddLifecycle(name string, lifecycle MachineDriverLifecycle)
}

type machineDriverLister struct {
	controller *machineDriverController
}

func (l *machineDriverLister) List(namespace string, selector labels.Selector) (ret []*MachineDriver, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*MachineDriver))
	})
	return
}

func (l *machineDriverLister) Get(namespace, name string) (*MachineDriver, error) {
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
			Group:    MachineDriverGroupVersionKind.Group,
			Resource: "machineDriver",
		}, name)
	}
	return obj.(*MachineDriver), nil
}

type machineDriverController struct {
	controller.GenericController
}

func (c *machineDriverController) Lister() MachineDriverLister {
	return &machineDriverLister{
		controller: c,
	}
}

func (c *machineDriverController) AddHandler(name string, handler MachineDriverHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*MachineDriver))
	})
}

type machineDriverFactory struct {
}

func (c machineDriverFactory) Object() runtime.Object {
	return &MachineDriver{}
}

func (c machineDriverFactory) List() runtime.Object {
	return &MachineDriverList{}
}

func (s *machineDriverClient) Controller() MachineDriverController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.machineDriverControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(MachineDriverGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &machineDriverController{
		GenericController: genericController,
	}

	s.client.machineDriverControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type machineDriverClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   MachineDriverController
}

func (s *machineDriverClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *machineDriverClient) Create(o *MachineDriver) (*MachineDriver, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*MachineDriver), err
}

func (s *machineDriverClient) Get(name string, opts metav1.GetOptions) (*MachineDriver, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*MachineDriver), err
}

func (s *machineDriverClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*MachineDriver, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*MachineDriver), err
}

func (s *machineDriverClient) Update(o *MachineDriver) (*MachineDriver, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*MachineDriver), err
}

func (s *machineDriverClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *machineDriverClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *machineDriverClient) List(opts metav1.ListOptions) (*MachineDriverList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*MachineDriverList), err
}

func (s *machineDriverClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *machineDriverClient) Patch(o *MachineDriver, data []byte, subresources ...string) (*MachineDriver, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*MachineDriver), err
}

func (s *machineDriverClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *machineDriverClient) AddHandler(name string, sync MachineDriverHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *machineDriverClient) AddLifecycle(name string, lifecycle MachineDriverLifecycle) {
	sync := NewMachineDriverLifecycleAdapter(name, s, lifecycle)
	s.AddHandler(name, sync)
}
