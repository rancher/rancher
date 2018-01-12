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
	MachineGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Machine",
	}
	MachineResource = metav1.APIResource{
		Name:         "machines",
		SingularName: "machine",
		Namespaced:   true,

		Kind: MachineGroupVersionKind.Kind,
	}
)

type MachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machine
}

type MachineHandlerFunc func(key string, obj *Machine) error

type MachineLister interface {
	List(namespace string, selector labels.Selector) (ret []*Machine, err error)
	Get(namespace, name string) (*Machine, error)
}

type MachineController interface {
	Informer() cache.SharedIndexInformer
	Lister() MachineLister
	AddHandler(handler MachineHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type MachineInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*Machine) (*Machine, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*Machine, error)
	Get(name string, opts metav1.GetOptions) (*Machine, error)
	Update(*Machine) (*Machine, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*MachineList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MachineController
	AddSyncHandler(sync MachineHandlerFunc)
	AddLifecycle(name string, lifecycle MachineLifecycle)
}

type machineLister struct {
	controller *machineController
}

func (l *machineLister) List(namespace string, selector labels.Selector) (ret []*Machine, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Machine))
	})
	return
}

func (l *machineLister) Get(namespace, name string) (*Machine, error) {
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
			Group:    MachineGroupVersionKind.Group,
			Resource: "machine",
		}, name)
	}
	return obj.(*Machine), nil
}

type machineController struct {
	controller.GenericController
}

func (c *machineController) Lister() MachineLister {
	return &machineLister{
		controller: c,
	}
}

func (c *machineController) AddHandler(handler MachineHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*Machine))
	})
}

type machineFactory struct {
}

func (c machineFactory) Object() runtime.Object {
	return &Machine{}
}

func (c machineFactory) List() runtime.Object {
	return &MachineList{}
}

func (s *machineClient) Controller() MachineController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.machineControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(MachineGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &machineController{
		GenericController: genericController,
	}

	s.client.machineControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type machineClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   MachineController
}

func (s *machineClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *machineClient) Create(o *Machine) (*Machine, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Machine), err
}

func (s *machineClient) Get(name string, opts metav1.GetOptions) (*Machine, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Machine), err
}

func (s *machineClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*Machine, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*Machine), err
}

func (s *machineClient) Update(o *Machine) (*Machine, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Machine), err
}

func (s *machineClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *machineClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *machineClient) List(opts metav1.ListOptions) (*MachineList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*MachineList), err
}

func (s *machineClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *machineClient) Patch(o *Machine, data []byte, subresources ...string) (*Machine, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*Machine), err
}

func (s *machineClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *machineClient) AddSyncHandler(sync MachineHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *machineClient) AddLifecycle(name string, lifecycle MachineLifecycle) {
	sync := NewMachineLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
