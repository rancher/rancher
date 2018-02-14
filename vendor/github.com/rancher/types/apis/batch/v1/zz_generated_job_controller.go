package v1

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type JobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Job
}

type JobHandlerFunc func(key string, obj *v1.Job) error

type JobLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Job, err error)
	Get(namespace, name string) (*v1.Job, error)
}

type JobController interface {
	Informer() cache.SharedIndexInformer
	Lister() JobLister
	AddHandler(name string, handler JobHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler JobHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type JobInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*v1.Job) (*v1.Job, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Job, error)
	Get(name string, opts metav1.GetOptions) (*v1.Job, error)
	Update(*v1.Job) (*v1.Job, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*JobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() JobController
	AddHandler(name string, sync JobHandlerFunc)
	AddLifecycle(name string, lifecycle JobLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync JobHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle JobLifecycle)
}

type jobLister struct {
	controller *jobController
}

func (l *jobLister) List(namespace string, selector labels.Selector) (ret []*v1.Job, err error) {
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
			Resource: "job",
		}, name)
	}
	return obj.(*v1.Job), nil
}

type jobController struct {
	controller.GenericController
}

func (c *jobController) Lister() JobLister {
	return &jobLister{
		controller: c,
	}
}

func (c *jobController) AddHandler(name string, handler JobHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.Job))
	})
}

func (c *jobController) AddClusterScopedHandler(name, cluster string, handler JobHandlerFunc) {
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

		return handler(key, obj.(*v1.Job))
	})
}

type jobFactory struct {
}

func (c jobFactory) Object() runtime.Object {
	return &v1.Job{}
}

func (c jobFactory) List() runtime.Object {
	return &JobList{}
}

func (s *jobClient) Controller() JobController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.jobControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(JobGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &jobController{
		GenericController: genericController,
	}

	s.client.jobControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type jobClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   JobController
}

func (s *jobClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *jobClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *jobClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *jobClient) List(opts metav1.ListOptions) (*JobList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*JobList), err
}

func (s *jobClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *jobClient) Patch(o *v1.Job, data []byte, subresources ...string) (*v1.Job, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.Job), err
}

func (s *jobClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *jobClient) AddHandler(name string, sync JobHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *jobClient) AddLifecycle(name string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *jobClient) AddClusterScopedHandler(name, clusterName string, sync JobHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *jobClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle JobLifecycle) {
	sync := NewJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
