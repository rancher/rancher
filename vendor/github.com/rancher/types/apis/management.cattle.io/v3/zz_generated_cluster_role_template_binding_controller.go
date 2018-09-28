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
	ClusterRoleTemplateBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRoleTemplateBinding",
	}
	ClusterRoleTemplateBindingResource = metav1.APIResource{
		Name:         "clusterroletemplatebindings",
		SingularName: "clusterroletemplatebinding",
		Namespaced:   true,

		Kind: ClusterRoleTemplateBindingGroupVersionKind.Kind,
	}
)

type ClusterRoleTemplateBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRoleTemplateBinding
}

type ClusterRoleTemplateBindingHandlerFunc func(key string, obj *ClusterRoleTemplateBinding) error

type ClusterRoleTemplateBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterRoleTemplateBinding, err error)
	Get(namespace, name string) (*ClusterRoleTemplateBinding, error)
}

type ClusterRoleTemplateBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleTemplateBindingLister
	AddHandler(name string, handler ClusterRoleTemplateBindingHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterRoleTemplateBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRoleTemplateBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error)
	Get(name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error)
	Update(*ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRoleTemplateBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleTemplateBindingController
	AddHandler(name string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterRoleTemplateBindingLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterRoleTemplateBindingHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterRoleTemplateBindingLifecycle)
}

type clusterRoleTemplateBindingLister struct {
	controller *clusterRoleTemplateBindingController
}

func (l *clusterRoleTemplateBindingLister) List(namespace string, selector labels.Selector) (ret []*ClusterRoleTemplateBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterRoleTemplateBinding))
	})
	return
}

func (l *clusterRoleTemplateBindingLister) Get(namespace, name string) (*ClusterRoleTemplateBinding, error) {
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
			Group:    ClusterRoleTemplateBindingGroupVersionKind.Group,
			Resource: "clusterRoleTemplateBinding",
		}, key)
	}
	return obj.(*ClusterRoleTemplateBinding), nil
}

type clusterRoleTemplateBindingController struct {
	controller.GenericController
}

func (c *clusterRoleTemplateBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterRoleTemplateBindingController) Lister() ClusterRoleTemplateBindingLister {
	return &clusterRoleTemplateBindingLister{
		controller: c,
	}
}

func (c *clusterRoleTemplateBindingController) AddHandler(name string, handler ClusterRoleTemplateBindingHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ClusterRoleTemplateBinding))
	})
}

func (c *clusterRoleTemplateBindingController) AddClusterScopedHandler(name, cluster string, handler ClusterRoleTemplateBindingHandlerFunc) {
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

		return handler(key, obj.(*ClusterRoleTemplateBinding))
	})
}

type clusterRoleTemplateBindingFactory struct {
}

func (c clusterRoleTemplateBindingFactory) Object() runtime.Object {
	return &ClusterRoleTemplateBinding{}
}

func (c clusterRoleTemplateBindingFactory) List() runtime.Object {
	return &ClusterRoleTemplateBindingList{}
}

func (s *clusterRoleTemplateBindingClient) Controller() ClusterRoleTemplateBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRoleTemplateBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRoleTemplateBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRoleTemplateBindingController{
		GenericController: genericController,
	}

	s.client.clusterRoleTemplateBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRoleTemplateBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterRoleTemplateBindingController
}

func (s *clusterRoleTemplateBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleTemplateBindingClient) Create(o *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) Get(name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) Update(o *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleTemplateBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterRoleTemplateBindingClient) List(opts metav1.ListOptions) (*ClusterRoleTemplateBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRoleTemplateBindingList), err
}

func (s *clusterRoleTemplateBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleTemplateBindingClient) Patch(o *ClusterRoleTemplateBinding, data []byte, subresources ...string) (*ClusterRoleTemplateBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ClusterRoleTemplateBinding), err
}

func (s *clusterRoleTemplateBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleTemplateBindingClient) AddHandler(name string, sync ClusterRoleTemplateBindingHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterRoleTemplateBindingClient) AddLifecycle(name string, lifecycle ClusterRoleTemplateBindingLifecycle) {
	sync := NewClusterRoleTemplateBindingLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterRoleTemplateBindingClient) AddClusterScopedHandler(name, clusterName string, sync ClusterRoleTemplateBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterRoleTemplateBindingClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterRoleTemplateBindingLifecycle) {
	sync := NewClusterRoleTemplateBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
