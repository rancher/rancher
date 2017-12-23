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
	PodSecurityPolicyTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicyTemplate",
	}
	PodSecurityPolicyTemplateResource = metav1.APIResource{
		Name:         "podsecuritypolicytemplates",
		SingularName: "podsecuritypolicytemplate",
		Namespaced:   false,
		Kind:         PodSecurityPolicyTemplateGroupVersionKind.Kind,
	}
)

type PodSecurityPolicyTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodSecurityPolicyTemplate
}

type PodSecurityPolicyTemplateHandlerFunc func(key string, obj *PodSecurityPolicyTemplate) error

type PodSecurityPolicyTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplate, err error)
	Get(namespace, name string) (*PodSecurityPolicyTemplate, error)
}

type PodSecurityPolicyTemplateController interface {
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyTemplateLister
	AddHandler(handler PodSecurityPolicyTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodSecurityPolicyTemplateInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error)
	Update(*PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyTemplateController
	AddSyncHandler(sync PodSecurityPolicyTemplateHandlerFunc)
	AddLifecycle(name string, lifecycle PodSecurityPolicyTemplateLifecycle)
}

type podSecurityPolicyTemplateLister struct {
	controller *podSecurityPolicyTemplateController
}

func (l *podSecurityPolicyTemplateLister) List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PodSecurityPolicyTemplate))
	})
	return
}

func (l *podSecurityPolicyTemplateLister) Get(namespace, name string) (*PodSecurityPolicyTemplate, error) {
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
			Group:    PodSecurityPolicyTemplateGroupVersionKind.Group,
			Resource: "podSecurityPolicyTemplate",
		}, name)
	}
	return obj.(*PodSecurityPolicyTemplate), nil
}

type podSecurityPolicyTemplateController struct {
	controller.GenericController
}

func (c *podSecurityPolicyTemplateController) Lister() PodSecurityPolicyTemplateLister {
	return &podSecurityPolicyTemplateLister{
		controller: c,
	}
}

func (c *podSecurityPolicyTemplateController) AddHandler(handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*PodSecurityPolicyTemplate))
	})
}

type podSecurityPolicyTemplateFactory struct {
}

func (c podSecurityPolicyTemplateFactory) Object() runtime.Object {
	return &PodSecurityPolicyTemplate{}
}

func (c podSecurityPolicyTemplateFactory) List() runtime.Object {
	return &PodSecurityPolicyTemplateList{}
}

func (s *podSecurityPolicyTemplateClient) Controller() PodSecurityPolicyTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podSecurityPolicyTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodSecurityPolicyTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podSecurityPolicyTemplateController{
		GenericController: genericController,
	}

	s.client.podSecurityPolicyTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podSecurityPolicyTemplateClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   PodSecurityPolicyTemplateController
}

func (s *podSecurityPolicyTemplateClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyTemplateClient) Create(o *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Update(o *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyTemplateClient) List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodSecurityPolicyTemplateList), err
}

func (s *podSecurityPolicyTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyTemplateClient) Patch(o *PodSecurityPolicyTemplate, data []byte, subresources ...string) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyTemplateClient) AddSyncHandler(sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *podSecurityPolicyTemplateClient) AddLifecycle(name string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
