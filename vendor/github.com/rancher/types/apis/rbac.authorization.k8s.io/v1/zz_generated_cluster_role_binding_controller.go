package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	ClusterRoleBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterRoleBinding",
	}
	ClusterRoleBindingResource = metav1.APIResource{
		Name:         "clusterrolebindings",
		SingularName: "clusterrolebinding",
		Namespaced:   false,
		Kind:         ClusterRoleBindingGroupVersionKind.Kind,
	}
)

type ClusterRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ClusterRoleBinding
}

type ClusterRoleBindingHandlerFunc func(key string, obj *v1.ClusterRoleBinding) error

type ClusterRoleBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ClusterRoleBinding, err error)
	Get(namespace, name string) (*v1.ClusterRoleBinding, error)
}

type ClusterRoleBindingController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterRoleBindingLister
	AddHandler(handler ClusterRoleBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterRoleBindingInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Get(name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	Update(*v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterRoleBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterRoleBindingController
	AddSyncHandler(sync ClusterRoleBindingHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterRoleBindingLifecycle)
}

type clusterRoleBindingLister struct {
	controller *clusterRoleBindingController
}

func (l *clusterRoleBindingLister) List(namespace string, selector labels.Selector) (ret []*v1.ClusterRoleBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ClusterRoleBinding))
	})
	return
}

func (l *clusterRoleBindingLister) Get(namespace, name string) (*v1.ClusterRoleBinding, error) {
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
			Group:    ClusterRoleBindingGroupVersionKind.Group,
			Resource: "clusterRoleBinding",
		}, name)
	}
	return obj.(*v1.ClusterRoleBinding), nil
}

type clusterRoleBindingController struct {
	controller.GenericController
}

func (c *clusterRoleBindingController) Lister() ClusterRoleBindingLister {
	return &clusterRoleBindingLister{
		controller: c,
	}
}

func (c *clusterRoleBindingController) AddHandler(handler ClusterRoleBindingHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.ClusterRoleBinding))
	})
}

type clusterRoleBindingFactory struct {
}

func (c clusterRoleBindingFactory) Object() runtime.Object {
	return &v1.ClusterRoleBinding{}
}

func (c clusterRoleBindingFactory) List() runtime.Object {
	return &ClusterRoleBindingList{}
}

func (s *clusterRoleBindingClient) Controller() ClusterRoleBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterRoleBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterRoleBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterRoleBindingController{
		GenericController: genericController,
	}

	s.client.clusterRoleBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterRoleBindingClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ClusterRoleBindingController
}

func (s *clusterRoleBindingClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *clusterRoleBindingClient) Create(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Get(name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Update(o *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterRoleBindingClient) List(opts metav1.ListOptions) (*ClusterRoleBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterRoleBindingList), err
}

func (s *clusterRoleBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterRoleBindingClient) Patch(o *v1.ClusterRoleBinding, data []byte, subresources ...string) (*v1.ClusterRoleBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.ClusterRoleBinding), err
}

func (s *clusterRoleBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterRoleBindingClient) AddSyncHandler(sync ClusterRoleBindingHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *clusterRoleBindingClient) AddLifecycle(name string, lifecycle ClusterRoleBindingLifecycle) {
	sync := NewClusterRoleBindingLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
