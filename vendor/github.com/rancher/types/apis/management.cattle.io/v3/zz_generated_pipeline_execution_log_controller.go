package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	PipelineExecutionLogGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PipelineExecutionLog",
	}
	PipelineExecutionLogResource = metav1.APIResource{
		Name:         "pipelineexecutionlogs",
		SingularName: "pipelineexecutionlog",
		Namespaced:   true,

		Kind: PipelineExecutionLogGroupVersionKind.Kind,
	}
)

type PipelineExecutionLogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineExecutionLog
}

type PipelineExecutionLogHandlerFunc func(key string, obj *PipelineExecutionLog) error

type PipelineExecutionLogLister interface {
	List(namespace string, selector labels.Selector) (ret []*PipelineExecutionLog, err error)
	Get(namespace, name string) (*PipelineExecutionLog, error)
}

type PipelineExecutionLogController interface {
	Informer() cache.SharedIndexInformer
	Lister() PipelineExecutionLogLister
	AddHandler(name string, handler PipelineExecutionLogHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler PipelineExecutionLogHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PipelineExecutionLogInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*PipelineExecutionLog) (*PipelineExecutionLog, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineExecutionLog, error)
	Get(name string, opts metav1.GetOptions) (*PipelineExecutionLog, error)
	Update(*PipelineExecutionLog) (*PipelineExecutionLog, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PipelineExecutionLogList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PipelineExecutionLogController
	AddHandler(name string, sync PipelineExecutionLogHandlerFunc)
	AddLifecycle(name string, lifecycle PipelineExecutionLogLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync PipelineExecutionLogHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle PipelineExecutionLogLifecycle)
}

type pipelineExecutionLogLister struct {
	controller *pipelineExecutionLogController
}

func (l *pipelineExecutionLogLister) List(namespace string, selector labels.Selector) (ret []*PipelineExecutionLog, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PipelineExecutionLog))
	})
	return
}

func (l *pipelineExecutionLogLister) Get(namespace, name string) (*PipelineExecutionLog, error) {
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
			Group:    PipelineExecutionLogGroupVersionKind.Group,
			Resource: "pipelineExecutionLog",
		}, key)
	}
	return obj.(*PipelineExecutionLog), nil
}

type pipelineExecutionLogController struct {
	controller.GenericController
}

func (c *pipelineExecutionLogController) Lister() PipelineExecutionLogLister {
	return &pipelineExecutionLogLister{
		controller: c,
	}
}

func (c *pipelineExecutionLogController) AddHandler(name string, handler PipelineExecutionLogHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*PipelineExecutionLog))
	})
}

func (c *pipelineExecutionLogController) AddClusterScopedHandler(name, cluster string, handler PipelineExecutionLogHandlerFunc) {
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

		return handler(key, obj.(*PipelineExecutionLog))
	})
}

type pipelineExecutionLogFactory struct {
}

func (c pipelineExecutionLogFactory) Object() runtime.Object {
	return &PipelineExecutionLog{}
}

func (c pipelineExecutionLogFactory) List() runtime.Object {
	return &PipelineExecutionLogList{}
}

func (s *pipelineExecutionLogClient) Controller() PipelineExecutionLogController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.pipelineExecutionLogControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PipelineExecutionLogGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &pipelineExecutionLogController{
		GenericController: genericController,
	}

	s.client.pipelineExecutionLogControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type pipelineExecutionLogClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PipelineExecutionLogController
}

func (s *pipelineExecutionLogClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *pipelineExecutionLogClient) Create(o *PipelineExecutionLog) (*PipelineExecutionLog, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PipelineExecutionLog), err
}

func (s *pipelineExecutionLogClient) Get(name string, opts metav1.GetOptions) (*PipelineExecutionLog, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PipelineExecutionLog), err
}

func (s *pipelineExecutionLogClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineExecutionLog, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PipelineExecutionLog), err
}

func (s *pipelineExecutionLogClient) Update(o *PipelineExecutionLog) (*PipelineExecutionLog, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PipelineExecutionLog), err
}

func (s *pipelineExecutionLogClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineExecutionLogClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineExecutionLogClient) List(opts metav1.ListOptions) (*PipelineExecutionLogList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PipelineExecutionLogList), err
}

func (s *pipelineExecutionLogClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineExecutionLogClient) Patch(o *PipelineExecutionLog, data []byte, subresources ...string) (*PipelineExecutionLog, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*PipelineExecutionLog), err
}

func (s *pipelineExecutionLogClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *pipelineExecutionLogClient) AddHandler(name string, sync PipelineExecutionLogHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *pipelineExecutionLogClient) AddLifecycle(name string, lifecycle PipelineExecutionLogLifecycle) {
	sync := NewPipelineExecutionLogLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *pipelineExecutionLogClient) AddClusterScopedHandler(name, clusterName string, sync PipelineExecutionLogHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *pipelineExecutionLogClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle PipelineExecutionLogLifecycle) {
	sync := NewPipelineExecutionLogLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
