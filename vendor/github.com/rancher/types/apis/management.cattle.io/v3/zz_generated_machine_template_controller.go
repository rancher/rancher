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
	MachineTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "MachineTemplate",
	}
	MachineTemplateResource = metav1.APIResource{
		Name:         "machinetemplates",
		SingularName: "machinetemplate",
		Namespaced:   false,
		Kind:         MachineTemplateGroupVersionKind.Kind,
	}
)

type MachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineTemplate
}

type MachineTemplateHandlerFunc func(key string, obj *MachineTemplate) error

type MachineTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*MachineTemplate, err error)
	Get(namespace, name string) (*MachineTemplate, error)
}

type MachineTemplateController interface {
	Informer() cache.SharedIndexInformer
	Lister() MachineTemplateLister
	AddHandler(handler MachineTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type MachineTemplateInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*MachineTemplate) (*MachineTemplate, error)
	Get(name string, opts metav1.GetOptions) (*MachineTemplate, error)
	Update(*MachineTemplate) (*MachineTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*MachineTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() MachineTemplateController
	AddSyncHandler(sync MachineTemplateHandlerFunc)
	AddLifecycle(name string, lifecycle MachineTemplateLifecycle)
}

type machineTemplateLister struct {
	controller *machineTemplateController
}

func (l *machineTemplateLister) List(namespace string, selector labels.Selector) (ret []*MachineTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*MachineTemplate))
	})
	return
}

func (l *machineTemplateLister) Get(namespace, name string) (*MachineTemplate, error) {
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
			Group:    MachineTemplateGroupVersionKind.Group,
			Resource: "machineTemplate",
		}, name)
	}
	return obj.(*MachineTemplate), nil
}

type machineTemplateController struct {
	controller.GenericController
}

func (c *machineTemplateController) Lister() MachineTemplateLister {
	return &machineTemplateLister{
		controller: c,
	}
}

func (c *machineTemplateController) AddHandler(handler MachineTemplateHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*MachineTemplate))
	})
}

type machineTemplateFactory struct {
}

func (c machineTemplateFactory) Object() runtime.Object {
	return &MachineTemplate{}
}

func (c machineTemplateFactory) List() runtime.Object {
	return &MachineTemplateList{}
}

func (s *machineTemplateClient) Controller() MachineTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.machineTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(MachineTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &machineTemplateController{
		GenericController: genericController,
	}

	s.client.machineTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type machineTemplateClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   MachineTemplateController
}

func (s *machineTemplateClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *machineTemplateClient) Create(o *MachineTemplate) (*MachineTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*MachineTemplate), err
}

func (s *machineTemplateClient) Get(name string, opts metav1.GetOptions) (*MachineTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*MachineTemplate), err
}

func (s *machineTemplateClient) Update(o *MachineTemplate) (*MachineTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*MachineTemplate), err
}

func (s *machineTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *machineTemplateClient) List(opts metav1.ListOptions) (*MachineTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*MachineTemplateList), err
}

func (s *machineTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *machineTemplateClient) Patch(o *MachineTemplate, data []byte, subresources ...string) (*MachineTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*MachineTemplate), err
}

func (s *machineTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *machineTemplateClient) AddSyncHandler(sync MachineTemplateHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *machineTemplateClient) AddLifecycle(name string, lifecycle MachineTemplateLifecycle) {
	sync := NewMachineTemplateLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
