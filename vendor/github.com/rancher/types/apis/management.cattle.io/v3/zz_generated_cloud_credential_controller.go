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
	CloudCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CloudCredential",
	}
	CloudCredentialResource = metav1.APIResource{
		Name:         "cloudcredentials",
		SingularName: "cloudcredential",
		Namespaced:   true,

		Kind: CloudCredentialGroupVersionKind.Kind,
	}
)

type CloudCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudCredential
}

type CloudCredentialHandlerFunc func(key string, obj *CloudCredential) error

type CloudCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*CloudCredential, err error)
	Get(namespace, name string) (*CloudCredential, error)
}

type CloudCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CloudCredentialLister
	AddHandler(name string, handler CloudCredentialHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler CloudCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CloudCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CloudCredential) (*CloudCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CloudCredential, error)
	Get(name string, opts metav1.GetOptions) (*CloudCredential, error)
	Update(*CloudCredential) (*CloudCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CloudCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CloudCredentialController
	AddHandler(name string, sync CloudCredentialHandlerFunc)
	AddLifecycle(name string, lifecycle CloudCredentialLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync CloudCredentialHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle CloudCredentialLifecycle)
}

type cloudCredentialLister struct {
	controller *cloudCredentialController
}

func (l *cloudCredentialLister) List(namespace string, selector labels.Selector) (ret []*CloudCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CloudCredential))
	})
	return
}

func (l *cloudCredentialLister) Get(namespace, name string) (*CloudCredential, error) {
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
			Group:    CloudCredentialGroupVersionKind.Group,
			Resource: "cloudCredential",
		}, key)
	}
	return obj.(*CloudCredential), nil
}

type cloudCredentialController struct {
	controller.GenericController
}

func (c *cloudCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cloudCredentialController) Lister() CloudCredentialLister {
	return &cloudCredentialLister{
		controller: c,
	}
}

func (c *cloudCredentialController) AddHandler(name string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*CloudCredential))
	})
}

func (c *cloudCredentialController) AddClusterScopedHandler(name, cluster string, handler CloudCredentialHandlerFunc) {
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

		return handler(key, obj.(*CloudCredential))
	})
}

type cloudCredentialFactory struct {
}

func (c cloudCredentialFactory) Object() runtime.Object {
	return &CloudCredential{}
}

func (c cloudCredentialFactory) List() runtime.Object {
	return &CloudCredentialList{}
}

func (s *cloudCredentialClient) Controller() CloudCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.cloudCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CloudCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &cloudCredentialController{
		GenericController: genericController,
	}

	s.client.cloudCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type cloudCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CloudCredentialController
}

func (s *cloudCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cloudCredentialClient) Create(o *CloudCredential) (*CloudCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) Get(name string, opts metav1.GetOptions) (*CloudCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CloudCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) Update(o *CloudCredential) (*CloudCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cloudCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cloudCredentialClient) List(opts metav1.ListOptions) (*CloudCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CloudCredentialList), err
}

func (s *cloudCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cloudCredentialClient) Patch(o *CloudCredential, data []byte, subresources ...string) (*CloudCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cloudCredentialClient) AddHandler(name string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *cloudCredentialClient) AddLifecycle(name string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *cloudCredentialClient) AddClusterScopedHandler(name, clusterName string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
