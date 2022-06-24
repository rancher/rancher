package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
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
	PodGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Pod",
	}
	PodResource = metav1.APIResource{
		Name:         "pods",
		SingularName: "pod",
		Namespaced:   true,

		Kind: PodGroupVersionKind.Kind,
	}

	PodGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "pods",
	}
)

func init() {
	resource.Put(PodGroupVersionResource)
}

// Deprecated: use v1.Pod instead
type Pod = v1.Pod

func NewPod(namespace, name string, obj v1.Pod) *v1.Pod {
	obj.APIVersion, obj.Kind = PodGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodHandlerFunc func(key string, obj *v1.Pod) (runtime.Object, error)

type PodChangeHandlerFunc func(obj *v1.Pod) (runtime.Object, error)

type PodLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Pod, err error)
	Get(namespace, name string) (*v1.Pod, error)
}

type PodController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodLister
	AddHandler(ctx context.Context, name string, handler PodHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type PodInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Pod) (*v1.Pod, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Pod, error)
	Get(name string, opts metav1.GetOptions) (*v1.Pod, error)
	Update(*v1.Pod) (*v1.Pod, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.PodList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.PodList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodController
	AddHandler(ctx context.Context, name string, sync PodHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodLifecycle)
}

type podLister struct {
	ns         string
	controller *podController
}

func (l *podLister) List(namespace string, selector labels.Selector) (ret []*v1.Pod, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Pod))
	})
	return
}

func (l *podLister) Get(namespace, name string) (*v1.Pod, error) {
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
			Group:    PodGroupVersionKind.Group,
			Resource: PodGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Pod), nil
}

type podController struct {
	ns string
	controller.GenericController
}

func (c *podController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podController) Lister() PodLister {
	return &podLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *podController) AddHandler(ctx context.Context, name string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Pod); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Pod); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Pod); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Pod); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podFactory struct {
}

func (c podFactory) Object() runtime.Object {
	return &v1.Pod{}
}

func (c podFactory) List() runtime.Object {
	return &v1.PodList{}
}

func (s *podClient) Controller() PodController {
	genericController := controller.NewGenericController(s.ns, PodGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(PodGroupVersionResource, PodGroupVersionKind.Kind, true))

	return &podController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type podClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodController
}

func (s *podClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podClient) Create(o *v1.Pod) (*v1.Pod, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Pod), err
}

func (s *podClient) Get(name string, opts metav1.GetOptions) (*v1.Pod, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Pod), err
}

func (s *podClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Pod, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Pod), err
}

func (s *podClient) Update(o *v1.Pod) (*v1.Pod, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Pod), err
}

func (s *podClient) UpdateStatus(o *v1.Pod) (*v1.Pod, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Pod), err
}

func (s *podClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podClient) List(opts metav1.ListOptions) (*v1.PodList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.PodList), err
}

func (s *podClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.PodList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.PodList), err
}

func (s *podClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podClient) Patch(o *v1.Pod, patchType types.PatchType, data []byte, subresources ...string) (*v1.Pod, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Pod), err
}

func (s *podClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podClient) AddHandler(ctx context.Context, name string, sync PodHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podClient) AddLifecycle(ctx context.Context, name string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
