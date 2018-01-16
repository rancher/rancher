package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
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
	Informer() cache.SharedIndexInformer
	Lister() DockerCredentialLister
	AddHandler(name string, handler DockerCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type DockerCredentialInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*DockerCredential) (*DockerCredential, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*DockerCredential, error)
	Get(name string, opts metav1.GetOptions) (*DockerCredential, error)
	Update(*DockerCredential) (*DockerCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*DockerCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() DockerCredentialController
	AddHandler(name string, sync DockerCredentialHandlerFunc)
	AddLifecycle(name string, lifecycle DockerCredentialLifecycle)
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
		}, name)
	}
	return obj.(*DockerCredential), nil
}

type dockerCredentialController struct {
	controller.GenericController
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
	objectClient *clientbase.ObjectClient
	controller   DockerCredentialController
}

func (s *dockerCredentialClient) ObjectClient() *clientbase.ObjectClient {
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

func (s *dockerCredentialClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*DockerCredential, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) Update(o *DockerCredential) (*DockerCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*DockerCredential), err
}

func (s *dockerCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *dockerCredentialClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
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
	sync := NewDockerCredentialLifecycleAdapter(name, s, lifecycle)
	s.AddHandler(name, sync)
}
