package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
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
	PersistentVolumeClaimGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PersistentVolumeClaim",
	}
	PersistentVolumeClaimResource = metav1.APIResource{
		Name:         "persistentvolumeclaims",
		SingularName: "persistentvolumeclaim",
		Namespaced:   true,

		Kind: PersistentVolumeClaimGroupVersionKind.Kind,
	}
)

type PersistentVolumeClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.PersistentVolumeClaim
}

type PersistentVolumeClaimHandlerFunc func(key string, obj *v1.PersistentVolumeClaim) error

type PersistentVolumeClaimLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.PersistentVolumeClaim, err error)
	Get(namespace, name string) (*v1.PersistentVolumeClaim, error)
}

type PersistentVolumeClaimController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PersistentVolumeClaimLister
	AddHandler(name string, handler PersistentVolumeClaimHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler PersistentVolumeClaimHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PersistentVolumeClaimInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error)
	Get(name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error)
	Update(*v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PersistentVolumeClaimList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PersistentVolumeClaimController
	AddHandler(name string, sync PersistentVolumeClaimHandlerFunc)
	AddLifecycle(name string, lifecycle PersistentVolumeClaimLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync PersistentVolumeClaimHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle PersistentVolumeClaimLifecycle)
}

type persistentVolumeClaimLister struct {
	controller *persistentVolumeClaimController
}

func (l *persistentVolumeClaimLister) List(namespace string, selector labels.Selector) (ret []*v1.PersistentVolumeClaim, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.PersistentVolumeClaim))
	})
	return
}

func (l *persistentVolumeClaimLister) Get(namespace, name string) (*v1.PersistentVolumeClaim, error) {
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
			Group:    PersistentVolumeClaimGroupVersionKind.Group,
			Resource: "persistentVolumeClaim",
		}, key)
	}
	return obj.(*v1.PersistentVolumeClaim), nil
}

type persistentVolumeClaimController struct {
	controller.GenericController
}

func (c *persistentVolumeClaimController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *persistentVolumeClaimController) Lister() PersistentVolumeClaimLister {
	return &persistentVolumeClaimLister{
		controller: c,
	}
}

func (c *persistentVolumeClaimController) AddHandler(name string, handler PersistentVolumeClaimHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*v1.PersistentVolumeClaim))
	})
}

func (c *persistentVolumeClaimController) AddClusterScopedHandler(name, cluster string, handler PersistentVolumeClaimHandlerFunc) {
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

		return handler(key, obj.(*v1.PersistentVolumeClaim))
	})
}

type persistentVolumeClaimFactory struct {
}

func (c persistentVolumeClaimFactory) Object() runtime.Object {
	return &v1.PersistentVolumeClaim{}
}

func (c persistentVolumeClaimFactory) List() runtime.Object {
	return &PersistentVolumeClaimList{}
}

func (s *persistentVolumeClaimClient) Controller() PersistentVolumeClaimController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.persistentVolumeClaimControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PersistentVolumeClaimGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &persistentVolumeClaimController{
		GenericController: genericController,
	}

	s.client.persistentVolumeClaimControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type persistentVolumeClaimClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PersistentVolumeClaimController
}

func (s *persistentVolumeClaimClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *persistentVolumeClaimClient) Create(o *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) Get(name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) Update(o *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *persistentVolumeClaimClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *persistentVolumeClaimClient) List(opts metav1.ListOptions) (*PersistentVolumeClaimList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PersistentVolumeClaimList), err
}

func (s *persistentVolumeClaimClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *persistentVolumeClaimClient) Patch(o *v1.PersistentVolumeClaim, data []byte, subresources ...string) (*v1.PersistentVolumeClaim, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*v1.PersistentVolumeClaim), err
}

func (s *persistentVolumeClaimClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *persistentVolumeClaimClient) AddHandler(name string, sync PersistentVolumeClaimHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *persistentVolumeClaimClient) AddLifecycle(name string, lifecycle PersistentVolumeClaimLifecycle) {
	sync := NewPersistentVolumeClaimLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *persistentVolumeClaimClient) AddClusterScopedHandler(name, clusterName string, sync PersistentVolumeClaimHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *persistentVolumeClaimClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle PersistentVolumeClaimLifecycle) {
	sync := NewPersistentVolumeClaimLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
