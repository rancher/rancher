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
	GithubCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: "v3",
		Group:   "management.cattle.io",
		Kind:    "GithubCredential",
	}
	GithubCredentialResource = metav1.APIResource{
		Name:         "githubcredentials",
		SingularName: "githubcredential",
		Namespaced:   false,
		Kind:         GithubCredentialGroupVersionKind.Kind,
	}
)

type GithubCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GithubCredential
}

type GithubCredentialHandlerFunc func(key string, obj *GithubCredential) error

type GithubCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*GithubCredential, err error)
	Get(namespace, name string) (*GithubCredential, error)
}

type GithubCredentialController interface {
	Informer() cache.SharedIndexInformer
	Lister() GithubCredentialLister
	AddHandler(handler GithubCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type GithubCredentialInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*GithubCredential) (*GithubCredential, error)
	Get(name string, opts metav1.GetOptions) (*GithubCredential, error)
	Update(*GithubCredential) (*GithubCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*GithubCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() GithubCredentialController
}

type githubCredentialLister struct {
	controller *githubCredentialController
}

func (l *githubCredentialLister) List(namespace string, selector labels.Selector) (ret []*GithubCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*GithubCredential))
	})
	return
}

func (l *githubCredentialLister) Get(namespace, name string) (*GithubCredential, error) {
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
			Group:    GithubCredentialGroupVersionKind.Group,
			Resource: "githubCredential",
		}, name)
	}
	return obj.(*GithubCredential), nil
}

type githubCredentialController struct {
	controller.GenericController
}

func (c *githubCredentialController) Lister() GithubCredentialLister {
	return &githubCredentialLister{
		controller: c,
	}
}

func (c *githubCredentialController) AddHandler(handler GithubCredentialHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*GithubCredential))
	})
}

type githubCredentialFactory struct {
}

func (c githubCredentialFactory) Object() runtime.Object {
	return &GithubCredential{}
}

func (c githubCredentialFactory) List() runtime.Object {
	return &GithubCredentialList{}
}

func (s *githubCredentialClient) Controller() GithubCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.githubCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(GithubCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &githubCredentialController{
		GenericController: genericController,
	}

	s.client.githubCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type githubCredentialClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   GithubCredentialController
}

func (s *githubCredentialClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *githubCredentialClient) Create(o *GithubCredential) (*GithubCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*GithubCredential), err
}

func (s *githubCredentialClient) Get(name string, opts metav1.GetOptions) (*GithubCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*GithubCredential), err
}

func (s *githubCredentialClient) Update(o *GithubCredential) (*GithubCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*GithubCredential), err
}

func (s *githubCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *githubCredentialClient) List(opts metav1.ListOptions) (*GithubCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*GithubCredentialList), err
}

func (s *githubCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

func (s *githubCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}
