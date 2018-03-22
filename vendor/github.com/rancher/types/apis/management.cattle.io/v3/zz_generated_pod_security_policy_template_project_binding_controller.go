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
	PodSecurityPolicyTemplateProjectBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicyTemplateProjectBinding",
	}
	PodSecurityPolicyTemplateProjectBindingResource = metav1.APIResource{
		Name:         "podsecuritypolicytemplateprojectbindings",
		SingularName: "podsecuritypolicytemplateprojectbinding",
		Namespaced:   true,

		Kind: PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Kind,
	}
)

type PodSecurityPolicyTemplateProjectBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodSecurityPolicyTemplateProjectBinding
}

type PodSecurityPolicyTemplateProjectBindingHandlerFunc func(key string, obj *PodSecurityPolicyTemplateProjectBinding) error

type PodSecurityPolicyTemplateProjectBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplateProjectBinding, err error)
	Get(namespace, name string) (*PodSecurityPolicyTemplateProjectBinding, error)
}

type PodSecurityPolicyTemplateProjectBindingController interface {
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyTemplateProjectBindingLister
	AddHandler(name string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodSecurityPolicyTemplateProjectBindingInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error)
	Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error)
	Update(*PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateProjectBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyTemplateProjectBindingController
	AddHandler(name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddLifecycle(name string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle)
}

type podSecurityPolicyTemplateProjectBindingLister struct {
	controller *podSecurityPolicyTemplateProjectBindingController
}

func (l *podSecurityPolicyTemplateProjectBindingLister) List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplateProjectBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PodSecurityPolicyTemplateProjectBinding))
	})
	return
}

func (l *podSecurityPolicyTemplateProjectBindingLister) Get(namespace, name string) (*PodSecurityPolicyTemplateProjectBinding, error) {
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
			Group:    PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Group,
			Resource: "podSecurityPolicyTemplateProjectBinding",
		}, name)
	}
	return obj.(*PodSecurityPolicyTemplateProjectBinding), nil
}

type podSecurityPolicyTemplateProjectBindingController struct {
	controller.GenericController
}

func (c *podSecurityPolicyTemplateProjectBindingController) Lister() PodSecurityPolicyTemplateProjectBindingLister {
	return &podSecurityPolicyTemplateProjectBindingLister{
		controller: c,
	}
}

func (c *podSecurityPolicyTemplateProjectBindingController) AddHandler(name string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*PodSecurityPolicyTemplateProjectBinding))
	})
}

func (c *podSecurityPolicyTemplateProjectBindingController) AddClusterScopedHandler(name, cluster string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
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

		return handler(key, obj.(*PodSecurityPolicyTemplateProjectBinding))
	})
}

type podSecurityPolicyTemplateProjectBindingFactory struct {
}

func (c podSecurityPolicyTemplateProjectBindingFactory) Object() runtime.Object {
	return &PodSecurityPolicyTemplateProjectBinding{}
}

func (c podSecurityPolicyTemplateProjectBindingFactory) List() runtime.Object {
	return &PodSecurityPolicyTemplateProjectBindingList{}
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Controller() PodSecurityPolicyTemplateProjectBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podSecurityPolicyTemplateProjectBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podSecurityPolicyTemplateProjectBindingController{
		GenericController: genericController,
	}

	s.client.podSecurityPolicyTemplateProjectBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podSecurityPolicyTemplateProjectBindingClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   PodSecurityPolicyTemplateProjectBindingController
}

func (s *podSecurityPolicyTemplateProjectBindingClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Create(o *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Update(o *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateProjectBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodSecurityPolicyTemplateProjectBindingList), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyTemplateProjectBindingClient) Patch(o *PodSecurityPolicyTemplateProjectBinding, data []byte, subresources ...string) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddHandler(name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddLifecycle(name string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle) {
	sync := NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddClusterScopedHandler(name, clusterName string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle) {
	sync := NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
