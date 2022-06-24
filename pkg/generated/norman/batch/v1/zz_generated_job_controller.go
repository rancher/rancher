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
	JobGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Job",
	}
	JobResource = metav1.APIResource{
		Name:         "jobs",
		SingularName: "job",
		Namespaced:   true,

		Kind: JobGroupVersionKind.Kind,
	}

	JobGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "jobs",
	}
)

func init() {
	resource.Put(JobGroupVersionResource)
}

// Deprecated: use v1.Job instead
type Job = v1.Job

func NewJob(namespace, name string, obj v1.Job) *v1.Job {
	obj.APIVersion, obj.Kind = JobGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type JobHandlerFunc func(key string, obj *v1.Job) (runtime.Object, error)

type JobChangeHandlerFunc func(obj *v1.Job) (runtime.Object, error)

type JobLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Job, err error)
	Get(namespace, name string) (*v1.Job, error)
}

type JobController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() JobLister
	AddHandler(ctx context.Context, name string, handler JobHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync JobHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler JobHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler JobHandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type JobInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Job) (*v1.Job, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Job, error)
	Get(name string, opts metav1.GetOptions) (*v1.Job, error)
	Update(*v1.Job) (*v1.Job, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*v1.JobList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.JobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() JobController
	AddHandler(ctx context.Context, name string, sync JobHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync JobHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle JobLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle JobLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync JobHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync JobHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle JobLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle JobLifecycle)
}

type jobLister struct {
	ns         string
	controller *jobController
}

func (l *jobLister) List(namespace string, selector labels.Selector) (ret []*v1.Job, err error) {
	if namespace == "" {
		namespace = l.ns
	}
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Job))
	})
	return
}

func (l *jobLister) Get(namespace, name string) (*v1.Job, error) {
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
			Group:    JobGroupVersionKind.Group,
			Resource: JobGroupVersionResource.Resource,
		}, key)
	}
	return obj.(*v1.Job), nil
}

type jobController struct {
	ns string
	controller.GenericController
}

func (c *jobController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *jobController) Lister() JobLister {
	return &jobLister{
		ns:         c.ns,
		controller: c,
	}
}

func (c *jobController) AddHandler(ctx context.Context, name string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Job); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *jobController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Job); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *jobController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Job); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *jobController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Job); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type jobFactory struct {
}

func (c jobFactory) Object() runtime.Object {
	return &v1.Job{}
}

func (c jobFactory) List() runtime.Object {
	return &v1.JobList{}
}

func (s *jobClient) Controller() JobController {
	genericController := controller.NewGenericController(s.ns, JobGroupVersionKind.Kind+"Controller",
		s.client.controllerFactory.ForResourceKind(JobGroupVersionResource, JobGroupVersionKind.Kind, true))

	return &jobController{
		ns:                s.ns,
		GenericController: genericController,
	}
}

type jobClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   JobController
}

func (s *jobClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *jobClient) Create(o *v1.Job) (*v1.Job, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Job), err
}

func (s *jobClient) Get(name string, opts metav1.GetOptions) (*v1.Job, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Job), err
}

func (s *jobClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Job, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Job), err
}

func (s *jobClient) Update(o *v1.Job) (*v1.Job, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Job), err
}

func (s *jobClient) UpdateStatus(o *v1.Job) (*v1.Job, error) {
	obj, err := s.objectClient.UpdateStatus(o.Name, o)
	return obj.(*v1.Job), err
}

func (s *jobClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *jobClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *jobClient) List(opts metav1.ListOptions) (*v1.JobList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*v1.JobList), err
}

func (s *jobClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*v1.JobList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*v1.JobList), err
}

func (s *jobClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *jobClient) Patch(o *v1.Job, patchType types.PatchType, data []byte, subresources ...string) (*v1.Job, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Job), err
}

func (s *jobClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *jobClient) AddHandler(ctx context.Context, name string, sync JobHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *jobClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync JobHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *jobClient) AddLifecycle(ctx context.Context, name string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *jobClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *jobClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync JobHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *jobClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync JobHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *jobClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *jobClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}
