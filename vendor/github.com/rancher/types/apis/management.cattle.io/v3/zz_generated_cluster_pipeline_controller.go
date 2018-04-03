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
	ClusterPipelineGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterPipeline",
	}
	ClusterPipelineResource = metav1.APIResource{
		Name:         "clusterpipelines",
		SingularName: "clusterpipeline",
		Namespaced:   true,

		Kind: ClusterPipelineGroupVersionKind.Kind,
	}
)

type ClusterPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPipeline
}

type ClusterPipelineHandlerFunc func(key string, obj *ClusterPipeline) error

type ClusterPipelineLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterPipeline, err error)
	Get(namespace, name string) (*ClusterPipeline, error)
}

type ClusterPipelineController interface {
	Informer() cache.SharedIndexInformer
	Lister() ClusterPipelineLister
	AddHandler(name string, handler ClusterPipelineHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler ClusterPipelineHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterPipelineInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterPipeline) (*ClusterPipeline, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterPipeline, error)
	Get(name string, opts metav1.GetOptions) (*ClusterPipeline, error)
	Update(*ClusterPipeline) (*ClusterPipeline, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterPipelineList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterPipelineController
	AddHandler(name string, sync ClusterPipelineHandlerFunc)
	AddLifecycle(name string, lifecycle ClusterPipelineLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync ClusterPipelineHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterPipelineLifecycle)
}

type clusterPipelineLister struct {
	controller *clusterPipelineController
}

func (l *clusterPipelineLister) List(namespace string, selector labels.Selector) (ret []*ClusterPipeline, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterPipeline))
	})
	return
}

func (l *clusterPipelineLister) Get(namespace, name string) (*ClusterPipeline, error) {
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
			Group:    ClusterPipelineGroupVersionKind.Group,
			Resource: "clusterPipeline",
		}, name)
	}
	return obj.(*ClusterPipeline), nil
}

type clusterPipelineController struct {
	controller.GenericController
}

func (c *clusterPipelineController) Lister() ClusterPipelineLister {
	return &clusterPipelineLister{
		controller: c,
	}
}

func (c *clusterPipelineController) AddHandler(name string, handler ClusterPipelineHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*ClusterPipeline))
	})
}

func (c *clusterPipelineController) AddClusterScopedHandler(name, cluster string, handler ClusterPipelineHandlerFunc) {
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

		return handler(key, obj.(*ClusterPipeline))
	})
}

type clusterPipelineFactory struct {
}

func (c clusterPipelineFactory) Object() runtime.Object {
	return &ClusterPipeline{}
}

func (c clusterPipelineFactory) List() runtime.Object {
	return &ClusterPipelineList{}
}

func (s *clusterPipelineClient) Controller() ClusterPipelineController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterPipelineControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterPipelineGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterPipelineController{
		GenericController: genericController,
	}

	s.client.clusterPipelineControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterPipelineClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterPipelineController
}

func (s *clusterPipelineClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterPipelineClient) Create(o *ClusterPipeline) (*ClusterPipeline, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterPipeline), err
}

func (s *clusterPipelineClient) Get(name string, opts metav1.GetOptions) (*ClusterPipeline, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterPipeline), err
}

func (s *clusterPipelineClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterPipeline, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterPipeline), err
}

func (s *clusterPipelineClient) Update(o *ClusterPipeline) (*ClusterPipeline, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterPipeline), err
}

func (s *clusterPipelineClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterPipelineClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterPipelineClient) List(opts metav1.ListOptions) (*ClusterPipelineList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterPipelineList), err
}

func (s *clusterPipelineClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterPipelineClient) Patch(o *ClusterPipeline, data []byte, subresources ...string) (*ClusterPipeline, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*ClusterPipeline), err
}

func (s *clusterPipelineClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterPipelineClient) AddHandler(name string, sync ClusterPipelineHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *clusterPipelineClient) AddLifecycle(name string, lifecycle ClusterPipelineLifecycle) {
	sync := NewClusterPipelineLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *clusterPipelineClient) AddClusterScopedHandler(name, clusterName string, sync ClusterPipelineHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *clusterPipelineClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle ClusterPipelineLifecycle) {
	sync := NewClusterPipelineLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
