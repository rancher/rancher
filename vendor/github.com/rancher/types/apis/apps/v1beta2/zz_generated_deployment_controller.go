package v1beta2

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	DeploymentGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Deployment",
	}
	DeploymentResource = metav1.APIResource{
		Name:         "deployments",
		SingularName: "deployment",
		Namespaced:   true,

		Kind: DeploymentGroupVersionKind.Kind,
	}
)

type DeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta2.Deployment
}

type DeploymentHandlerFunc func(key string, obj *v1beta2.Deployment) error

type DeploymentLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta2.Deployment, err error)
	Get(namespace, name string) (*v1beta2.Deployment, error)
}

type DeploymentController interface {
	Informer() cache.SharedIndexInformer
	Lister() DeploymentLister
	AddHandler(name string, handler DeploymentHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler DeploymentHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DeploymentInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1beta2.Deployment) (*v1beta2.Deployment, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.Deployment, error)
	Get(name string, opts metav1.GetOptions) (*v1beta2.Deployment, error)
	Update(*v1beta2.Deployment) (*v1beta2.Deployment, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DeploymentList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DeploymentController
	AddHandler(name string, sync DeploymentHandlerFunc)
	AddLifecycle(name string, lifecycle DeploymentLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync DeploymentHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle DeploymentLifecycle)
}

type deploymentLister struct {
	controller *deploymentController
}

func (l *deploymentLister) List(namespace string, selector labels.Selector) (ret []*v1beta2.Deployment, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta2.Deployment))
	})
	return
}

func (l *deploymentLister) Get(namespace, name string) (*v1beta2.Deployment, error) {
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
			Group:    DeploymentGroupVersionKind.Group,
			Resource: "deployment",
		}, name)
	}
	return obj.(*v1beta2.Deployment), nil
}

type deploymentController struct {
	controller.GenericController
}

func (c *deploymentController) Lister() DeploymentLister {
	return &deploymentLister{
		controller: c,
	}
}

func (c *deploymentController) AddHandler(name string, handler DeploymentHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1beta2.Deployment))
	})
}

func (c *deploymentController) AddClusterScopedHandler(name, cluster string, handler DeploymentHandlerFunc) {
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

		return handler(key, obj.(*v1beta2.Deployment))
	})
}

type deploymentFactory struct {
}

func (c deploymentFactory) Object() runtime.Object {
	return &v1beta2.Deployment{}
}

func (c deploymentFactory) List() runtime.Object {
	return &DeploymentList{}
}

func (s *deploymentClient) Controller() DeploymentController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.deploymentControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DeploymentGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &deploymentController{
		GenericController: genericController,
	}

	s.client.deploymentControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type deploymentClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DeploymentController
}

func (s *deploymentClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *deploymentClient) Create(o *v1beta2.Deployment) (*v1beta2.Deployment, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta2.Deployment), err
}

func (s *deploymentClient) Get(name string, opts metav1.GetOptions) (*v1beta2.Deployment, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta2.Deployment), err
}

func (s *deploymentClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta2.Deployment, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta2.Deployment), err
}

func (s *deploymentClient) Update(o *v1beta2.Deployment) (*v1beta2.Deployment, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta2.Deployment), err
}

func (s *deploymentClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *deploymentClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *deploymentClient) List(opts metav1.ListOptions) (*DeploymentList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DeploymentList), err
}

func (s *deploymentClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *deploymentClient) Patch(o *v1beta2.Deployment, data []byte, subresources ...string) (*v1beta2.Deployment, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1beta2.Deployment), err
}

func (s *deploymentClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *deploymentClient) AddHandler(name string, sync DeploymentHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *deploymentClient) AddLifecycle(name string, lifecycle DeploymentLifecycle) {
	sync := NewDeploymentLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *deploymentClient) AddClusterScopedHandler(name, clusterName string, sync DeploymentHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *deploymentClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle DeploymentLifecycle) {
	sync := NewDeploymentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
