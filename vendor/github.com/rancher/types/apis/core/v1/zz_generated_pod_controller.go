package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type PodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Pod
}

type PodHandlerFunc func(key string, obj *v1.Pod) error

type PodLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Pod, err error)
	Get(namespace, name string) (*v1.Pod, error)
}

type PodController interface {
	Informer() cache.SharedIndexInformer
	Lister() PodLister
	AddHandler(name string, handler PodHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler PodHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.Pod) (*v1.Pod, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Pod, error)
	Get(name string, opts metav1.GetOptions) (*v1.Pod, error)
	Update(*v1.Pod) (*v1.Pod, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodController
	AddHandler(name string, sync PodHandlerFunc)
	AddLifecycle(name string, lifecycle PodLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync PodHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle PodLifecycle)
}

type podLister struct {
	controller *podController
}

func (l *podLister) List(namespace string, selector labels.Selector) (ret []*v1.Pod, err error) {
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
			Resource: "pod",
		}, name)
	}
	return obj.(*v1.Pod), nil
}

type podController struct {
	controller.GenericController
}

func (c *podController) Lister() PodLister {
	return &podLister{
		controller: c,
	}
}

func (c *podController) AddHandler(name string, handler PodHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Pod))
	})
}

func (c *podController) AddClusterScopedHandler(name, cluster string, handler PodHandlerFunc) {
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

		return handler(key, obj.(*v1.Pod))
	})
}

type podFactory struct {
}

func (c podFactory) Object() runtime.Object {
	return &v1.Pod{}
}

func (c podFactory) List() runtime.Object {
	return &PodList{}
}

func (s *podClient) Controller() PodController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podController{
		GenericController: genericController,
	}

	s.client.podControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   PodController
}

func (s *podClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *podClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*v1.Pod, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*v1.Pod), err
}

func (s *podClient) Update(o *v1.Pod) (*v1.Pod, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Pod), err
}

func (s *podClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *podClient) List(opts metav1.ListOptions) (*PodList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodList), err
}

func (s *podClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podClient) Patch(o *v1.Pod, data []byte, subresources ...string) (*v1.Pod, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.Pod), err
}

func (s *podClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podClient) AddHandler(name string, sync PodHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *podClient) AddLifecycle(name string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *podClient) AddClusterScopedHandler(name, clusterName string, sync PodHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *podClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle PodLifecycle) {
	sync := NewPodLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
