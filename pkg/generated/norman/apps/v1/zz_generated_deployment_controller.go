package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

	DeploymentGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "deployments",
	}
)

func init() {
	resource.Put(DeploymentGroupVersionResource)
}

// Deprecated: use v1.Deployment instead
type Deployment = v1.Deployment

func NewDeployment(namespace, name string, obj v1.Deployment) *v1.Deployment {
	obj.APIVersion, obj.Kind = DeploymentGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type DeploymentHandlerFunc func(key string, obj *v1.Deployment) (runtime.Object, error)

type DeploymentChangeHandlerFunc func(obj *v1.Deployment) (runtime.Object, error)

type DeploymentLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Deployment, err error)
	Get(namespace, name string) (*v1.Deployment, error)
}

type DeploymentController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DeploymentLister
	AddHandler(ctx context.Context, name string, handler DeploymentHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DeploymentHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler DeploymentHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler DeploymentHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type DeploymentInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Deployment) (*v1.Deployment, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Deployment, error)
	Get(name string, opts metav1.GetOptions) (*v1.Deployment, error)
	Update(*v1.Deployment) (*v1.Deployment, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.DeploymentList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.DeploymentList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DeploymentController
	AddHandler(ctx context.Context, name string, sync DeploymentHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DeploymentHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle DeploymentLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DeploymentLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DeploymentHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DeploymentHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DeploymentLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DeploymentLifecycle)
}

type deploymentLister struct {
	ns         string
	controller *deploymentController
}

func (l *deploymentLister) List(namespace string, selector labels.Selector) (ret []*v1.Deployment, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Deployment))
	})
	return
}

func (l *deploymentLister) Get(namespace, name string) (*v1.Deployment, error) {
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
			Resource: DeploymentGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Deployment), nil
}

type deploymentController struct {
	ns string
	controller.GenericController
}

func (c *deploymentController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *deploymentController) Lister() DeploymentLister {
	return &deploymentLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *deploymentController) AddHandler(ctx context.Context, name string, handler DeploymentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Deployment); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *deploymentController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler DeploymentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Deployment); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *deploymentController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler DeploymentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Deployment); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *deploymentController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler DeploymentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Deployment); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type deploymentFactory struct {
}

func (c deploymentFactory) Object() runtime.Object {
	return &v1.Deployment{}
}

func (c deploymentFactory) List() runtime.Object {
	return &v1.DeploymentList{}
}

func (s *deploymentClient) Controller() DeploymentController {
	genericController := controller.NewGenericController(s.ns, DeploymentGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(DeploymentGroupVersionResource, DeploymentGroupVersionKind.Kind, true))

	return &deploymentController{
		ns:                s.ns,
		GenericController: genericController,
	}
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

func (s *deploymentClient) Create(o *v1.Deployment) (*v1.Deployment, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Deployment), err
}

func (s *deploymentClient) Get(name string, opts metav1.GetOptions) (*v1.Deployment, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Deployment), err
}

func (s *deploymentClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Deployment, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Deployment), err
}

func (s *deploymentClient) Update(o *v1.Deployment) (*v1.Deployment, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Deployment), err
}

func (s *deploymentClient) UpdateStatus(o *v1.Deployment) (*v1.Deployment, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Deployment), err
}

func (s *deploymentClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *deploymentClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *deploymentClient) List(opts metav1.ListOptions) (*v1.DeploymentList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.DeploymentList), err
}

func (s *deploymentClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.DeploymentList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.DeploymentList), err
}

func (s *deploymentClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *deploymentClient) Patch(o *v1.Deployment, patchType types.PatchType, data []byte, subresources ...string) (*v1.Deployment, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Deployment), err
}

func (s *deploymentClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *deploymentClient) AddHandler(ctx context.Context, name string, sync DeploymentHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *deploymentClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync DeploymentHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *deploymentClient) AddLifecycle(ctx context.Context, name string, lifecycle DeploymentLifecycle) {
	sync := NewDeploymentLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *deploymentClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle DeploymentLifecycle) {
	sync := NewDeploymentLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *deploymentClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync DeploymentHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *deploymentClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync DeploymentHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *deploymentClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle DeploymentLifecycle) {
	sync := NewDeploymentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *deploymentClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle DeploymentLifecycle) {
	sync := NewDeploymentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
