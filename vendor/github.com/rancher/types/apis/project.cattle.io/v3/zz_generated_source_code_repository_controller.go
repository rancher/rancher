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
	SourceCodeRepositoryGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeRepository",
	}
	SourceCodeRepositoryResource = metav1.APIResource{
		Name:         "sourcecoderepositories",
		SingularName: "sourcecoderepository",
		Namespaced:   true,

		Kind: SourceCodeRepositoryGroupVersionKind.Kind,
	}
)

type SourceCodeRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeRepository
}

type SourceCodeRepositoryHandlerFunc func(key string, obj *SourceCodeRepository) error

type SourceCodeRepositoryLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeRepository, err error)
	Get(namespace, name string) (*SourceCodeRepository, error)
}

type SourceCodeRepositoryController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeRepositoryLister
	AddHandler(name string, handler SourceCodeRepositoryHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler SourceCodeRepositoryHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeRepositoryInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeRepository) (*SourceCodeRepository, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeRepository, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeRepository, error)
	Update(*SourceCodeRepository) (*SourceCodeRepository, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeRepositoryList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeRepositoryController
	AddHandler(name string, sync SourceCodeRepositoryHandlerFunc)
	AddLifecycle(name string, lifecycle SourceCodeRepositoryLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync SourceCodeRepositoryHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeRepositoryLifecycle)
}

type sourceCodeRepositoryLister struct {
	controller *sourceCodeRepositoryController
}

func (l *sourceCodeRepositoryLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeRepository, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeRepository))
	})
	return
}

func (l *sourceCodeRepositoryLister) Get(namespace, name string) (*SourceCodeRepository, error) {
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
			Group:    SourceCodeRepositoryGroupVersionKind.Group,
			Resource: "sourceCodeRepository",
		}, key)
	}
	return obj.(*SourceCodeRepository), nil
}

type sourceCodeRepositoryController struct {
	controller.GenericController
}

func (c *sourceCodeRepositoryController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeRepositoryController) Lister() SourceCodeRepositoryLister {
	return &sourceCodeRepositoryLister{
		controller: c,
	}
}

func (c *sourceCodeRepositoryController) AddHandler(name string, handler SourceCodeRepositoryHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*SourceCodeRepository))
	})
}

func (c *sourceCodeRepositoryController) AddClusterScopedHandler(name, cluster string, handler SourceCodeRepositoryHandlerFunc) {
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

		return handler(key, obj.(*SourceCodeRepository))
	})
}

type sourceCodeRepositoryFactory struct {
}

func (c sourceCodeRepositoryFactory) Object() runtime.Object {
	return &SourceCodeRepository{}
}

func (c sourceCodeRepositoryFactory) List() runtime.Object {
	return &SourceCodeRepositoryList{}
}

func (s *sourceCodeRepositoryClient) Controller() SourceCodeRepositoryController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeRepositoryControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeRepositoryGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeRepositoryController{
		GenericController: genericController,
	}

	s.client.sourceCodeRepositoryControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeRepositoryClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeRepositoryController
}

func (s *sourceCodeRepositoryClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeRepositoryClient) Create(o *SourceCodeRepository) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) Get(name string, opts metav1.GetOptions) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) Update(o *SourceCodeRepository) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeRepositoryClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeRepositoryClient) List(opts metav1.ListOptions) (*SourceCodeRepositoryList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeRepositoryList), err
}

func (s *sourceCodeRepositoryClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeRepositoryClient) Patch(o *SourceCodeRepository, data []byte, subresources ...string) (*SourceCodeRepository, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*SourceCodeRepository), err
}

func (s *sourceCodeRepositoryClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeRepositoryClient) AddHandler(name string, sync SourceCodeRepositoryHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *sourceCodeRepositoryClient) AddLifecycle(name string, lifecycle SourceCodeRepositoryLifecycle) {
	sync := NewSourceCodeRepositoryLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *sourceCodeRepositoryClient) AddClusterScopedHandler(name, clusterName string, sync SourceCodeRepositoryHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *sourceCodeRepositoryClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle SourceCodeRepositoryLifecycle) {
	sync := NewSourceCodeRepositoryLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
