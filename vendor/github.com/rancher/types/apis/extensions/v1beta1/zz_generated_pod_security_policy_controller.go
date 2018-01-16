package v1beta1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	PodSecurityPolicyGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicy",
	}
	PodSecurityPolicyResource = metav1.APIResource{
		Name:         "podsecuritypolicies",
		SingularName: "podsecuritypolicy",
		Namespaced:   false,
		Kind:         PodSecurityPolicyGroupVersionKind.Kind,
	}
)

type PodSecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta1.PodSecurityPolicy
}

type PodSecurityPolicyHandlerFunc func(key string, obj *v1beta1.PodSecurityPolicy) error

type PodSecurityPolicyLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error)
	Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error)
}

type PodSecurityPolicyController interface {
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyLister
	AddHandler(name string, handler PodSecurityPolicyHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodSecurityPolicyInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Get(name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Update(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodSecurityPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyController
	AddHandler(name string, sync PodSecurityPolicyHandlerFunc)
	AddLifecycle(name string, lifecycle PodSecurityPolicyLifecycle)
}

type podSecurityPolicyLister struct {
	controller *podSecurityPolicyController
}

func (l *podSecurityPolicyLister) List(namespace string, selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta1.PodSecurityPolicy))
	})
	return
}

func (l *podSecurityPolicyLister) Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error) {
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
			Group:    PodSecurityPolicyGroupVersionKind.Group,
			Resource: "podSecurityPolicy",
		}, name)
	}
	return obj.(*v1beta1.PodSecurityPolicy), nil
}

type podSecurityPolicyController struct {
	controller.GenericController
}

func (c *podSecurityPolicyController) Lister() PodSecurityPolicyLister {
	return &podSecurityPolicyLister{
		controller: c,
	}
}

func (c *podSecurityPolicyController) AddHandler(name string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1beta1.PodSecurityPolicy))
	})
}

type podSecurityPolicyFactory struct {
}

func (c podSecurityPolicyFactory) Object() runtime.Object {
	return &v1beta1.PodSecurityPolicy{}
}

func (c podSecurityPolicyFactory) List() runtime.Object {
	return &PodSecurityPolicyList{}
}

func (s *podSecurityPolicyClient) Controller() PodSecurityPolicyController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podSecurityPolicyControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodSecurityPolicyGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podSecurityPolicyController{
		GenericController: genericController,
	}

	s.client.podSecurityPolicyControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podSecurityPolicyClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   PodSecurityPolicyController
}

func (s *podSecurityPolicyClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyClient) Create(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Get(name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Update(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *podSecurityPolicyClient) List(opts metav1.ListOptions) (*PodSecurityPolicyList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodSecurityPolicyList), err
}

func (s *podSecurityPolicyClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyClient) Patch(o *v1beta1.PodSecurityPolicy, data []byte, subresources ...string) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyClient) AddHandler(name string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *podSecurityPolicyClient) AddLifecycle(name string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name, s, lifecycle)
	s.AddHandler(name, sync)
}
