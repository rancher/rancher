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
	SSHAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SSHAuth",
	}
	SSHAuthResource = metav1.APIResource{
		Name:         "sshauths",
		SingularName: "sshauth",
		Namespaced:   true,

		Kind: SSHAuthGroupVersionKind.Kind,
	}
)

type SSHAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHAuth
}

type SSHAuthHandlerFunc func(key string, obj *SSHAuth) error

type SSHAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*SSHAuth, err error)
	Get(namespace, name string) (*SSHAuth, error)
}

type SSHAuthController interface {
	Informer() cache.SharedIndexInformer
	Lister() SSHAuthLister
	AddHandler(handler SSHAuthHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SSHAuthInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*SSHAuth) (*SSHAuth, error)
	GetNamespace(name, namespace string, opts metav1.GetOptions) (*SSHAuth, error)
	Get(name string, opts metav1.GetOptions) (*SSHAuth, error)
	Update(*SSHAuth) (*SSHAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SSHAuthController
	AddSyncHandler(sync SSHAuthHandlerFunc)
	AddLifecycle(name string, lifecycle SSHAuthLifecycle)
}

type sshAuthLister struct {
	controller *sshAuthController
}

func (l *sshAuthLister) List(namespace string, selector labels.Selector) (ret []*SSHAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SSHAuth))
	})
	return
}

func (l *sshAuthLister) Get(namespace, name string) (*SSHAuth, error) {
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
			Group:    SSHAuthGroupVersionKind.Group,
			Resource: "sshAuth",
		}, name)
	}
	return obj.(*SSHAuth), nil
}

type sshAuthController struct {
	controller.GenericController
}

func (c *sshAuthController) Lister() SSHAuthLister {
	return &sshAuthLister{
		controller: c,
	}
}

func (c *sshAuthController) AddHandler(handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*SSHAuth))
	})
}

type sshAuthFactory struct {
}

func (c sshAuthFactory) Object() runtime.Object {
	return &SSHAuth{}
}

func (c sshAuthFactory) List() runtime.Object {
	return &SSHAuthList{}
}

func (s *sshAuthClient) Controller() SSHAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sshAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SSHAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sshAuthController{
		GenericController: genericController,
	}

	s.client.sshAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sshAuthClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   SSHAuthController
}

func (s *sshAuthClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *sshAuthClient) Create(o *SSHAuth) (*SSHAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Get(name string, opts metav1.GetOptions) (*SSHAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) GetNamespace(name, namespace string, opts metav1.GetOptions) (*SSHAuth, error) {
	obj, err := s.objectClient.GetNamespace(name, namespace, opts)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Update(o *SSHAuth) (*SSHAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sshAuthClient) DeleteNamespace(name, namespace string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespace(name, namespace, options)
}

func (s *sshAuthClient) List(opts metav1.ListOptions) (*SSHAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SSHAuthList), err
}

func (s *sshAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sshAuthClient) Patch(o *SSHAuth, data []byte, subresources ...string) (*SSHAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sshAuthClient) AddSyncHandler(sync SSHAuthHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *sshAuthClient) AddLifecycle(name string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
