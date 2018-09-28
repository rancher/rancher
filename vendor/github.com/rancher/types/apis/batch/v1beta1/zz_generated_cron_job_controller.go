package v1beta1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
)

type CronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta1.CronJob
}

type CronJobHandlerFunc func(key string, obj *v1beta1.CronJob) error

type CronJobLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta1.CronJob, err error)
	Get(namespace, name string) (*v1beta1.CronJob, error)
}

type CronJobController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CronJobLister
	AddHandler(name string, handler CronJobHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler CronJobHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CronJobInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1beta1.CronJob) (*v1beta1.CronJob, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.CronJob, error)
	Get(name string, opts metav1.GetOptions) (*v1beta1.CronJob, error)
	Update(*v1beta1.CronJob) (*v1beta1.CronJob, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CronJobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CronJobController
	AddHandler(name string, sync CronJobHandlerFunc)
	AddLifecycle(name string, lifecycle CronJobLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync CronJobHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle CronJobLifecycle)
}

type cronJobLister struct {
	controller *cronJobController
}

func (l *cronJobLister) List(namespace string, selector labels.Selector) (ret []*v1beta1.CronJob, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta1.CronJob))
	})
	return
}

func (l *cronJobLister) Get(namespace, name string) (*v1beta1.CronJob, error) {
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
			Resource: "cronJob",
		}, key)
	}
	return obj.(*v1beta1.CronJob), nil
}

type cronJobController struct {
	controller.GenericController
}

func (c *cronJobController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cronJobController) Lister() CronJobLister {
	return &cronJobLister{
		controller: c,
	}
}

func (c *cronJobController) AddHandler(name string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1beta1.CronJob))
	})
}

func (c *cronJobController) AddClusterScopedHandler(name, cluster string, handler CronJobHandlerFunc) {
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

		return handler(key, obj.(*v1beta1.CronJob))
	})
}

type cronJobFactory struct {
}

func (c cronJobFactory) Object() runtime.Object {
	return &v1beta1.CronJob{}
}

func (c cronJobFactory) List() runtime.Object {
	return &CronJobList{}
}

func (s *cronJobClient) Controller() CronJobController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.cronJobControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CronJobGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &cronJobController{
		GenericController: genericController,
	}

	s.client.cronJobControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
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

func (s *cronJobClient) Create(o *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) Get(name string, opts metav1.GetOptions) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) Update(o *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cronJobClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cronJobClient) List(opts metav1.ListOptions) (*CronJobList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CronJobList), err
}

func (s *cronJobClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cronJobClient) Patch(o *v1beta1.CronJob, data []byte, subresources ...string) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cronJobClient) AddHandler(name string, sync CronJobHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *cronJobClient) AddLifecycle(name string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *cronJobClient) AddClusterScopedHandler(name, clusterName string, sync CronJobHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
