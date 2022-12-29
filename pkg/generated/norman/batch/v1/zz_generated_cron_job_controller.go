package v1

import (
	"context"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/batch/v1"
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
	CronJobGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CronJob",
	}
	CronJobResource = metav1.APIResource{
		Name:         "cronjobs",
		SingularName: "cronjob",
		Namespaced:   true,

		Kind: CronJobGroupVersionKind.Kind,
	}

	CronJobGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "cronjobs",
	}
)

func init() {
	resource.Put(CronJobGroupVersionResource)
}

// Deprecated: use v1.CronJob instead
type CronJob = v1.CronJob

func NewCronJob(namespace, name string, obj v1.CronJob) *v1.CronJob {
	obj.APIVersion, obj.Kind = CronJobGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CronJobHandlerFunc func(key string, obj *v1.CronJob) (runtime.Object, error)

type CronJobChangeHandlerFunc func(obj *v1.CronJob) (runtime.Object, error)

type CronJobLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.CronJob, err error)
	Get(namespace, name string) (*v1.CronJob, error)
}

type CronJobController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CronJobLister
	AddHandler(ctx context.Context, name string, handler CronJobHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CronJobHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CronJobHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CronJobHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type CronJobInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.CronJob) (*v1.CronJob, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.CronJob, error)
	Get(name string, opts metav1.GetOptions) (*v1.CronJob, error)
	Update(*v1.CronJob) (*v1.CronJob, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.CronJobList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.CronJobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CronJobController
	AddHandler(ctx context.Context, name string, sync CronJobHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CronJobHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CronJobLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CronJobLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CronJobHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CronJobHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CronJobLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CronJobLifecycle)
}

type cronJobLister struct {
	ns         string
	controller *cronJobController
}

func (l *cronJobLister) List(namespace string, selector labels.Selector) (ret []*v1.CronJob, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.CronJob))
	})
	return
}

func (l *cronJobLister) Get(namespace, name string) (*v1.CronJob, error) {
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
			Group:    CronJobGroupVersionKind.Group,
			Resource: CronJobGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.CronJob), nil
}

type cronJobController struct {
	ns string
	controller.GenericController
}

func (c *cronJobController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cronJobController) Lister() CronJobLister {
	return &cronJobLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *cronJobController) AddHandler(ctx context.Context, name string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.CronJob); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cronJobController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.CronJob); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cronJobController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.CronJob); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cronJobController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.CronJob); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type cronJobFactory struct {
}

func (c cronJobFactory) Object() runtime.Object {
	return &v1.CronJob{}
}

func (c cronJobFactory) List() runtime.Object {
	return &v1.CronJobList{}
}

func (s *cronJobClient) Controller() CronJobController {
	genericController := controller.NewGenericController(s.ns, CronJobGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(CronJobGroupVersionResource, CronJobGroupVersionKind.Kind, true))

	return &cronJobController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type cronJobClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CronJobController
}

func (s *cronJobClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cronJobClient) Create(o *v1.CronJob) (*v1.CronJob, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.CronJob), err
}

func (s *cronJobClient) Get(name string, opts metav1.GetOptions) (*v1.CronJob, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.CronJob), err
}

func (s *cronJobClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.CronJob, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.CronJob), err
}

func (s *cronJobClient) Update(o *v1.CronJob) (*v1.CronJob, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.CronJob), err
}

func (s *cronJobClient) UpdateStatus(o *v1.CronJob) (*v1.CronJob, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.CronJob), err
}

func (s *cronJobClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cronJobClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cronJobClient) List(opts metav1.ListOptions) (*v1.CronJobList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.CronJobList), err
}

func (s *cronJobClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.CronJobList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.CronJobList), err
}

func (s *cronJobClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cronJobClient) Patch(o *v1.CronJob, patchType types.PatchType, data []byte, subresources ...string) (*v1.CronJob, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.CronJob), err
}

func (s *cronJobClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cronJobClient) AddHandler(ctx context.Context, name string, sync CronJobHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cronJobClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CronJobHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cronJobClient) AddLifecycle(ctx context.Context, name string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cronJobClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cronJobClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CronJobHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CronJobHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
