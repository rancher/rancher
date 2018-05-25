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
	CattleInstanceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CattleInstance",
	}
	CattleInstanceResource = metav1.APIResource{
		Name:         "cattleinstances",
		SingularName: "cattleinstance",
		Namespaced:   false,
		Kind:         CattleInstanceGroupVersionKind.Kind,
	}
)

type CattleInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CattleInstance
}

type CattleInstanceHandlerFunc func(key string, obj *CattleInstance) error

type CattleInstanceLister interface {
	List(namespace string, selector labels.Selector) (ret []*CattleInstance, err error)
	Get(namespace, name string) (*CattleInstance, error)
}

type CattleInstanceController interface {
	Informer() cache.SharedIndexInformer
	Lister() CattleInstanceLister
	AddHandler(name string, handler CattleInstanceHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler CattleInstanceHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CattleInstanceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CattleInstance) (*CattleInstance, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CattleInstance, error)
	Get(name string, opts metav1.GetOptions) (*CattleInstance, error)
	Update(*CattleInstance) (*CattleInstance, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CattleInstanceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CattleInstanceController
	AddHandler(name string, sync CattleInstanceHandlerFunc)
	AddLifecycle(name string, lifecycle CattleInstanceLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync CattleInstanceHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle CattleInstanceLifecycle)
}

type cattleInstanceLister struct {
	controller *cattleInstanceController
}

func (l *cattleInstanceLister) List(namespace string, selector labels.Selector) (ret []*CattleInstance, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CattleInstance))
	})
	return
}

func (l *cattleInstanceLister) Get(namespace, name string) (*CattleInstance, error) {
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
			Group:    CattleInstanceGroupVersionKind.Group,
			Resource: "cattleInstance",
		}, key)
	}
	return obj.(*CattleInstance), nil
}

type cattleInstanceController struct {
	controller.GenericController
}

func (c *cattleInstanceController) Lister() CattleInstanceLister {
	return &cattleInstanceLister{
		controller: c,
	}
}

func (c *cattleInstanceController) AddHandler(name string, handler CattleInstanceHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*CattleInstance))
	})
}

func (c *cattleInstanceController) AddClusterScopedHandler(name, cluster string, handler CattleInstanceHandlerFunc) {
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

		return handler(key, obj.(*CattleInstance))
	})
}

type cattleInstanceFactory struct {
}

func (c cattleInstanceFactory) Object() runtime.Object {
	return &CattleInstance{}
}

func (c cattleInstanceFactory) List() runtime.Object {
	return &CattleInstanceList{}
}

func (s *cattleInstanceClient) Controller() CattleInstanceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.cattleInstanceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CattleInstanceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &cattleInstanceController{
		GenericController: genericController,
	}

	s.client.cattleInstanceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type cattleInstanceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CattleInstanceController
}

func (s *cattleInstanceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cattleInstanceClient) Create(o *CattleInstance) (*CattleInstance, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CattleInstance), err
}

func (s *cattleInstanceClient) Get(name string, opts metav1.GetOptions) (*CattleInstance, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CattleInstance), err
}

func (s *cattleInstanceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CattleInstance, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CattleInstance), err
}

func (s *cattleInstanceClient) Update(o *CattleInstance) (*CattleInstance, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CattleInstance), err
}

func (s *cattleInstanceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cattleInstanceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cattleInstanceClient) List(opts metav1.ListOptions) (*CattleInstanceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CattleInstanceList), err
}

func (s *cattleInstanceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cattleInstanceClient) Patch(o *CattleInstance, data []byte, subresources ...string) (*CattleInstance, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*CattleInstance), err
}

func (s *cattleInstanceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cattleInstanceClient) AddHandler(name string, sync CattleInstanceHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *cattleInstanceClient) AddLifecycle(name string, lifecycle CattleInstanceLifecycle) {
	sync := NewCattleInstanceLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *cattleInstanceClient) AddClusterScopedHandler(name, clusterName string, sync CattleInstanceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *cattleInstanceClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle CattleInstanceLifecycle) {
	sync := NewCattleInstanceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
