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
	DockerCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "DockerCredential",
	}
	DockerCredentialResource = metav1.APIResource{
		Name:         "dockercredentials",
		SingularName: "dockercredential",
		Namespaced:   true,

		Kind: DockerCredentialGroupVersionKind.Kind,
	}
)

type DockerCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DockerCredential
}

type DockerCredentialHandlerFunc func(key string, obj *DockerCredential) error

type DockerCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*DockerCredential, err error)
	Get(namespace, name string) (*DockerCredential, error)
}

type DockerCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() DockerCredentialLister
	AddHandler(name string, handler DockerCredentialHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler DockerCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DockerCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*DockerCredential) (*DockerCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*DockerCredential, error)
	Get(name string, opts metav1.GetOptions) (*DockerCredential, error)
	Update(*DockerCredential) (*DockerCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DockerCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DockerCredentialController
	AddHandler(name string, sync DockerCredentialHandlerFunc)
	AddLifecycle(name string, lifecycle DockerCredentialLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync DockerCredentialHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle DockerCredentialLifecycle)
}

type dockerCredentialLister struct {
	controller *dockerCredentialController
}

func (l *dockerCredentialLister) List(namespace string, selector labels.Selector) (ret []*DockerCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*DockerCredential))
	})
	return
}

func (l *dockerCredentialLister) Get(namespace, name string) (*DockerCredential, error) {
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
			Group:    DockerCredentialGroupVersionKind.Group,
			Resource: "dockerCredential",
		}, key)
	}
	return obj.(*DockerCredential), nil
}

type dockerCredentialController struct {
	controller.GenericController
}

func (c *dockerCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *dockerCredentialController) Lister() DockerCredentialLister {
	return &dockerCredentialLister{
		controller: c,
	}
}

func (c *dockerCredentialController) AddHandler(name string, handler DockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*DockerCredential))
	})
}

func (c *dockerCredentialController) AddClusterScopedHandler(name, cluster string, handler DockerCredentialHandlerFunc) {
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

		return handler(key, obj.(*DockerCredential))
	})
}

type dockerCredentialFactory struct {
}

func (c dockerCredentialFactory) Object() runtime.Object {
	return &DockerCredential{}
}

func (c dockerCredentialFactory) List() runtime.Object {
	return &DockerCredentialList{}
}

func (s *dockerCredentialClient) Controller() DockerCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.dockerCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(DockerCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &dockerCredentialController{
		GenericController: genericController,
	}

	s.client.dockerCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type dockerCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   DockerCredentialController
}

func (s *dockerCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *dockerCredentialClient) Create(o *DockerCredential) (*DockerCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) Get(name string, opts metav1.GetOptions) (*DockerCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*DockerCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) Update(o *DockerCredential) (*DockerCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *dockerCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *dockerCredentialClient) List(opts metav1.ListOptions) (*DockerCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*DockerCredentialList), err
}

func (s *dockerCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *dockerCredentialClient) Patch(o *DockerCredential, data []byte, subresources ...string) (*DockerCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *dockerCredentialClient) AddHandler(name string, sync DockerCredentialHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *dockerCredentialClient) AddLifecycle(name string, lifecycle DockerCredentialLifecycle) {
	sync := NewDockerCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *dockerCredentialClient) AddClusterScopedHandler(name, clusterName string, sync DockerCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *dockerCredentialClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle DockerCredentialLifecycle) {
	sync := NewDockerCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
