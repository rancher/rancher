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
	ProjectNetworkPolicyGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectNetworkPolicy",
	}
	ProjectNetworkPolicyResource = metav1.APIResource{
		Name:         "projectnetworkpolicies",
		SingularName: "projectnetworkpolicy",
		Namespaced:   true,

		Kind: ProjectNetworkPolicyGroupVersionKind.Kind,
	}
)

type ProjectNetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectNetworkPolicy
}

type ProjectNetworkPolicyHandlerFunc func(key string, obj *ProjectNetworkPolicy) error

type ProjectNetworkPolicyLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectNetworkPolicy, err error)
	Get(namespace, name string) (*ProjectNetworkPolicy, error)
}

type ProjectNetworkPolicyController interface {
	Informer() cache.SharedIndexInformer
	Lister() ProjectNetworkPolicyLister
	AddHandler(name string, handler ProjectNetworkPolicyHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ProjectNetworkPolicyHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectNetworkPolicyInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error)
	Get(name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error)
	Update(*ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectNetworkPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectNetworkPolicyController
	AddHandler(name string, sync ProjectNetworkPolicyHandlerFunc)
	AddLifecycle(name string, lifecycle ProjectNetworkPolicyLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ProjectNetworkPolicyHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectNetworkPolicyLifecycle)
}

type projectNetworkPolicyLister struct {
	controller *projectNetworkPolicyController
}

func (l *projectNetworkPolicyLister) List(namespace string, selector labels.Selector) (ret []*ProjectNetworkPolicy, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectNetworkPolicy))
	})
	return
}

func (l *projectNetworkPolicyLister) Get(namespace, name string) (*ProjectNetworkPolicy, error) {
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
			Group:    ProjectNetworkPolicyGroupVersionKind.Group,
			Resource: "projectNetworkPolicy",
		}, name)
	}
	return obj.(*ProjectNetworkPolicy), nil
}

type projectNetworkPolicyController struct {
	controller.GenericController
}

func (c *projectNetworkPolicyController) Lister() ProjectNetworkPolicyLister {
	return &projectNetworkPolicyLister{
		controller: c,
	}
}

func (c *projectNetworkPolicyController) AddHandler(name string, handler ProjectNetworkPolicyHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ProjectNetworkPolicy))
	})
}

func (c *projectNetworkPolicyController) AddClusterScopedHandler(name, cluster string, handler ProjectNetworkPolicyHandlerFunc) {
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

		return handler(key, obj.(*ProjectNetworkPolicy))
	})
}

type projectNetworkPolicyFactory struct {
}

func (c projectNetworkPolicyFactory) Object() runtime.Object {
	return &ProjectNetworkPolicy{}
}

func (c projectNetworkPolicyFactory) List() runtime.Object {
	return &ProjectNetworkPolicyList{}
}

func (s *projectNetworkPolicyClient) Controller() ProjectNetworkPolicyController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectNetworkPolicyControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectNetworkPolicyGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectNetworkPolicyController{
		GenericController: genericController,
	}

	s.client.projectNetworkPolicyControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectNetworkPolicyClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   ProjectNetworkPolicyController
}

func (s *projectNetworkPolicyClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *projectNetworkPolicyClient) Create(o *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) Get(name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) Update(o *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectNetworkPolicyClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectNetworkPolicyClient) List(opts metav1.ListOptions) (*ProjectNetworkPolicyList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectNetworkPolicyList), err
}

func (s *projectNetworkPolicyClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectNetworkPolicyClient) Patch(o *ProjectNetworkPolicy, data []byte, subresources ...string) (*ProjectNetworkPolicy, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ProjectNetworkPolicy), err
}

func (s *projectNetworkPolicyClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectNetworkPolicyClient) AddHandler(name string, sync ProjectNetworkPolicyHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *projectNetworkPolicyClient) AddLifecycle(name string, lifecycle ProjectNetworkPolicyLifecycle) {
	sync := NewProjectNetworkPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *projectNetworkPolicyClient) AddClusterScopedHandler(name, clusterName string, sync ProjectNetworkPolicyHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *projectNetworkPolicyClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ProjectNetworkPolicyLifecycle) {
	sync := NewProjectNetworkPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
