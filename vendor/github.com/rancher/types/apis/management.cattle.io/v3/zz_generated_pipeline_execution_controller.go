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
	PipelineExecutionGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PipelineExecution",
	}
	PipelineExecutionResource = metav1.APIResource{
		Name:         "pipelineexecutions",
		SingularName: "pipelineexecution",
		Namespaced:   true,

		Kind: PipelineExecutionGroupVersionKind.Kind,
	}
)

type PipelineExecutionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineExecution
}

type PipelineExecutionHandlerFunc func(key string, obj *PipelineExecution) error

type PipelineExecutionLister interface {
	List(namespace string, selector labels.Selector) (ret []*PipelineExecution, err error)
	Get(namespace, name string) (*PipelineExecution, error)
}

type PipelineExecutionController interface {
	Informer() cache.SharedIndexInformer
	Lister() PipelineExecutionLister
	AddHandler(name string, handler PipelineExecutionHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler PipelineExecutionHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PipelineExecutionInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*PipelineExecution) (*PipelineExecution, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineExecution, error)
	Get(name string, opts metav1.GetOptions) (*PipelineExecution, error)
	Update(*PipelineExecution) (*PipelineExecution, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PipelineExecutionList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PipelineExecutionController
	AddHandler(name string, sync PipelineExecutionHandlerFunc)
	AddLifecycle(name string, lifecycle PipelineExecutionLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync PipelineExecutionHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle PipelineExecutionLifecycle)
}

type pipelineExecutionLister struct {
	controller *pipelineExecutionController
}

func (l *pipelineExecutionLister) List(namespace string, selector labels.Selector) (ret []*PipelineExecution, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PipelineExecution))
	})
	return
}

func (l *pipelineExecutionLister) Get(namespace, name string) (*PipelineExecution, error) {
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
			Group:    PipelineExecutionGroupVersionKind.Group,
			Resource: "pipelineExecution",
		}, name)
	}
	return obj.(*PipelineExecution), nil
}

type pipelineExecutionController struct {
	controller.GenericController
}

func (c *pipelineExecutionController) Lister() PipelineExecutionLister {
	return &pipelineExecutionLister{
		controller: c,
	}
}

func (c *pipelineExecutionController) AddHandler(name string, handler PipelineExecutionHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*PipelineExecution))
	})
}

func (c *pipelineExecutionController) AddClusterScopedHandler(name, cluster string, handler PipelineExecutionHandlerFunc) {
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

		return handler(key, obj.(*PipelineExecution))
	})
}

type pipelineExecutionFactory struct {
}

func (c pipelineExecutionFactory) Object() runtime.Object {
	return &PipelineExecution{}
}

func (c pipelineExecutionFactory) List() runtime.Object {
	return &PipelineExecutionList{}
}

func (s *pipelineExecutionClient) Controller() PipelineExecutionController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.pipelineExecutionControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PipelineExecutionGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &pipelineExecutionController{
		GenericController: genericController,
	}

	s.client.pipelineExecutionControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type pipelineExecutionClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PipelineExecutionController
}

func (s *pipelineExecutionClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *pipelineExecutionClient) Create(o *PipelineExecution) (*PipelineExecution, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) Get(name string, opts metav1.GetOptions) (*PipelineExecution, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PipelineExecution, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) Update(o *PipelineExecution) (*PipelineExecution, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *pipelineExecutionClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *pipelineExecutionClient) List(opts metav1.ListOptions) (*PipelineExecutionList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PipelineExecutionList), err
}

func (s *pipelineExecutionClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *pipelineExecutionClient) Patch(o *PipelineExecution, data []byte, subresources ...string) (*PipelineExecution, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*PipelineExecution), err
}

func (s *pipelineExecutionClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *pipelineExecutionClient) AddHandler(name string, sync PipelineExecutionHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *pipelineExecutionClient) AddLifecycle(name string, lifecycle PipelineExecutionLifecycle) {
	sync := NewPipelineExecutionLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *pipelineExecutionClient) AddClusterScopedHandler(name, clusterName string, sync PipelineExecutionHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *pipelineExecutionClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle PipelineExecutionLifecycle) {
	sync := NewPipelineExecutionLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
