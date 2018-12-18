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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	TemplateContentGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "TemplateContent",
	}
	TemplateContentResource = metav1.APIResource{
		Name:         "templatecontents",
		SingularName: "templatecontent",
		Namespaced:   false,
		Kind:         TemplateContentGroupVersionKind.Kind,
	}
)

func NewTemplateContent(namespace, name string, obj TemplateContent) *TemplateContent {
	obj.APIVersion, obj.Kind = TemplateContentGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TemplateContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateContent
}

type TemplateContentHandlerFunc func(key string, obj *TemplateContent) (runtime.Object, error)

type TemplateContentChangeHandlerFunc func(obj *TemplateContent) (runtime.Object, error)

type TemplateContentLister interface {
	List(namespace string, selector labels.Selector) (ret []*TemplateContent, err error)
	Get(namespace, name string) (*TemplateContent, error)
}

type TemplateContentController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TemplateContentLister
	AddHandler(ctx context.Context, name string, handler TemplateContentHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TemplateContentHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateContentInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*TemplateContent) (*TemplateContent, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*TemplateContent, error)
	Get(name string, opts metav1.GetOptions) (*TemplateContent, error)
	Update(*TemplateContent) (*TemplateContent, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateContentList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateContentController
	AddHandler(ctx context.Context, name string, sync TemplateContentHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TemplateContentLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateContentHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateContentLifecycle)
}

type templateContentLister struct {
	controller *templateContentController
}

func (l *templateContentLister) List(namespace string, selector labels.Selector) (ret []*TemplateContent, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*TemplateContent))
	})
	return
}

func (l *templateContentLister) Get(namespace, name string) (*TemplateContent, error) {
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
			Group:    TemplateContentGroupVersionKind.Group,
			Resource: "templateContent",
		}, key)
	}
	return obj.(*TemplateContent), nil
}

type templateContentController struct {
	controller.GenericController
}

func (c *templateContentController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *templateContentController) Lister() TemplateContentLister {
	return &templateContentLister{
		controller: c,
	}
}

func (c *templateContentController) AddHandler(ctx context.Context, name string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*TemplateContent); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateContentController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TemplateContentHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*TemplateContent); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type templateContentFactory struct {
}

func (c templateContentFactory) Object() runtime.Object {
	return &TemplateContent{}
}

func (c templateContentFactory) List() runtime.Object {
	return &TemplateContentList{}
}

func (s *templateContentClient) Controller() TemplateContentController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.templateContentControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TemplateContentGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &templateContentController{
		GenericController: genericController,
	}

	s.client.templateContentControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type templateContentClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TemplateContentController
}

func (s *templateContentClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *templateContentClient) Create(o *TemplateContent) (*TemplateContent, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) Get(name string, opts metav1.GetOptions) (*TemplateContent, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*TemplateContent, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) Update(o *TemplateContent) (*TemplateContent, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateContentClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *templateContentClient) List(opts metav1.ListOptions) (*TemplateContentList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateContentList), err
}

func (s *templateContentClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateContentClient) Patch(o *TemplateContent, patchType types.PatchType, data []byte, subresources ...string) (*TemplateContent, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*TemplateContent), err
}

func (s *templateContentClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateContentClient) AddHandler(ctx context.Context, name string, sync TemplateContentHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateContentClient) AddLifecycle(ctx context.Context, name string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateContentClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateContentHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateContentClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateContentLifecycle) {
	sync := NewTemplateContentLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type TemplateContentIndexer func(obj *TemplateContent) ([]string, error)

type TemplateContentClientCache interface {
	Get(namespace, name string) (*TemplateContent, error)
	List(namespace string, selector labels.Selector) ([]*TemplateContent, error)

	Index(name string, indexer TemplateContentIndexer)
	GetIndexed(name, key string) ([]*TemplateContent, error)
}

type TemplateContentClient interface {
	Create(*TemplateContent) (*TemplateContent, error)
	Get(namespace, name string, opts metav1.GetOptions) (*TemplateContent, error)
	Update(*TemplateContent) (*TemplateContent, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*TemplateContentList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() TemplateContentClientCache

	OnCreate(ctx context.Context, name string, sync TemplateContentChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync TemplateContentChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync TemplateContentChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() TemplateContentInterface
}

type templateContentClientCache struct {
	client *templateContentClient2
}

type templateContentClient2 struct {
	iface      TemplateContentInterface
	controller TemplateContentController
}

func (n *templateContentClient2) Interface() TemplateContentInterface {
	return n.iface
}

func (n *templateContentClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *templateContentClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *templateContentClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *templateContentClient2) Create(obj *TemplateContent) (*TemplateContent, error) {
	return n.iface.Create(obj)
}

func (n *templateContentClient2) Get(namespace, name string, opts metav1.GetOptions) (*TemplateContent, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *templateContentClient2) Update(obj *TemplateContent) (*TemplateContent, error) {
	return n.iface.Update(obj)
}

func (n *templateContentClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *templateContentClient2) List(namespace string, opts metav1.ListOptions) (*TemplateContentList, error) {
	return n.iface.List(opts)
}

func (n *templateContentClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *templateContentClientCache) Get(namespace, name string) (*TemplateContent, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *templateContentClientCache) List(namespace string, selector labels.Selector) ([]*TemplateContent, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *templateContentClient2) Cache() TemplateContentClientCache {
	n.loadController()
	return &templateContentClientCache{
		client: n,
	}
}

func (n *templateContentClient2) OnCreate(ctx context.Context, name string, sync TemplateContentChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &templateContentLifecycleDelegate{create: sync})
}

func (n *templateContentClient2) OnChange(ctx context.Context, name string, sync TemplateContentChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &templateContentLifecycleDelegate{update: sync})
}

func (n *templateContentClient2) OnRemove(ctx context.Context, name string, sync TemplateContentChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &templateContentLifecycleDelegate{remove: sync})
}

func (n *templateContentClientCache) Index(name string, indexer TemplateContentIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*TemplateContent); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *templateContentClientCache) GetIndexed(name, key string) ([]*TemplateContent, error) {
	var result []*TemplateContent
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*TemplateContent); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *templateContentClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type templateContentLifecycleDelegate struct {
	create TemplateContentChangeHandlerFunc
	update TemplateContentChangeHandlerFunc
	remove TemplateContentChangeHandlerFunc
}

func (n *templateContentLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *templateContentLifecycleDelegate) Create(obj *TemplateContent) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *templateContentLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *templateContentLifecycleDelegate) Remove(obj *TemplateContent) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *templateContentLifecycleDelegate) Updated(obj *TemplateContent) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
