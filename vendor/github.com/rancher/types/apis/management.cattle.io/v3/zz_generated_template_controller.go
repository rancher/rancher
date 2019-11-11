package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
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
	TemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Template",
	}
	TemplateResource = metav1.APIResource{
		Name:         "templates",
		SingularName: "template",
		Namespaced:   false,
		Kind:         TemplateGroupVersionKind.Kind,
	}

	TemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "templates",
	}
)

func init() {
	resource.Put(TemplateGroupVersionResource)
}

func NewTemplate(namespace, name string, obj Template) *Template {
	obj.APIVersion, obj.Kind = TemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

type TemplateHandlerFunc func(key string, obj *Template) (runtime.Object, error)

type TemplateChangeHandlerFunc func(obj *Template) (runtime.Object, error)

type TemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*Template, err error)
	Get(namespace, name string) (*Template, error)
}

type TemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() TemplateLister
	AddHandler(ctx context.Context, name string, handler TemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler TemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler TemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type TemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Template) (*Template, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Template, error)
	Get(name string, opts metav1.GetOptions) (*Template, error)
	Update(*Template) (*Template, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*TemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*TemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() TemplateController
	AddHandler(ctx context.Context, name string, sync TemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle TemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateLifecycle)
}

type templateLister struct {
	controller *templateController
}

func (l *templateLister) List(namespace string, selector labels.Selector) (ret []*Template, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Template))
	})
	return
}

func (l *templateLister) Get(namespace, name string) (*Template, error) {
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
			Group:    TemplateGroupVersionKind.Group,
			Resource: "template",
		}, key)
	}
	return obj.(*Template), nil
}

type templateController struct {
	controller.GenericController
}

func (c *templateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *templateController) Lister() TemplateLister {
	return &templateLister{
		controller: c,
	}
}

func (c *templateController) AddHandler(ctx context.Context, name string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *templateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler TemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Template); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type templateFactory struct {
}

func (c templateFactory) Object() runtime.Object {
	return &Template{}
}

func (c templateFactory) List() runtime.Object {
	return &TemplateList{}
}

func (s *templateClient) Controller() TemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.templateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(TemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &templateController{
		GenericController: genericController,
	}

	s.client.templateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type templateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   TemplateController
}

func (s *templateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *templateClient) Create(o *Template) (*Template, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Template), err
}

func (s *templateClient) Get(name string, opts metav1.GetOptions) (*Template, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Template), err
}

func (s *templateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Template, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Template), err
}

func (s *templateClient) Update(o *Template) (*Template, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Template), err
}

func (s *templateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *templateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *templateClient) List(opts metav1.ListOptions) (*TemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*TemplateList), err
}

func (s *templateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*TemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*TemplateList), err
}

func (s *templateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *templateClient) Patch(o *Template, patchType types.PatchType, data []byte, subresources ...string) (*Template, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Template), err
}

func (s *templateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *templateClient) AddHandler(ctx context.Context, name string, sync TemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync TemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateClient) AddLifecycle(ctx context.Context, name string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *templateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *templateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync TemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync TemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *templateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *templateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle TemplateLifecycle) {
	sync := NewTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type TemplateIndexer func(obj *Template) ([]string, error)

type TemplateClientCache interface {
	Get(namespace, name string) (*Template, error)
	List(namespace string, selector labels.Selector) ([]*Template, error)

	Index(name string, indexer TemplateIndexer)
	GetIndexed(name, key string) ([]*Template, error)
}

type TemplateClient interface {
	Create(*Template) (*Template, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Template, error)
	Update(*Template) (*Template, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*TemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() TemplateClientCache

	OnCreate(ctx context.Context, name string, sync TemplateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync TemplateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync TemplateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() TemplateInterface
}

type templateClientCache struct {
	client *templateClient2
}

type templateClient2 struct {
	iface      TemplateInterface
	controller TemplateController
}

func (n *templateClient2) Interface() TemplateInterface {
	return n.iface
}

func (n *templateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *templateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *templateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *templateClient2) Create(obj *Template) (*Template, error) {
	return n.iface.Create(obj)
}

func (n *templateClient2) Get(namespace, name string, opts metav1.GetOptions) (*Template, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *templateClient2) Update(obj *Template) (*Template, error) {
	return n.iface.Update(obj)
}

func (n *templateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *templateClient2) List(namespace string, opts metav1.ListOptions) (*TemplateList, error) {
	return n.iface.List(opts)
}

func (n *templateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *templateClientCache) Get(namespace, name string) (*Template, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *templateClientCache) List(namespace string, selector labels.Selector) ([]*Template, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *templateClient2) Cache() TemplateClientCache {
	n.loadController()
	return &templateClientCache{
		client: n,
	}
}

func (n *templateClient2) OnCreate(ctx context.Context, name string, sync TemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &templateLifecycleDelegate{create: sync})
}

func (n *templateClient2) OnChange(ctx context.Context, name string, sync TemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &templateLifecycleDelegate{update: sync})
}

func (n *templateClient2) OnRemove(ctx context.Context, name string, sync TemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &templateLifecycleDelegate{remove: sync})
}

func (n *templateClientCache) Index(name string, indexer TemplateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Template); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *templateClientCache) GetIndexed(name, key string) ([]*Template, error) {
	var result []*Template
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Template); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *templateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type templateLifecycleDelegate struct {
	create TemplateChangeHandlerFunc
	update TemplateChangeHandlerFunc
	remove TemplateChangeHandlerFunc
}

func (n *templateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *templateLifecycleDelegate) Create(obj *Template) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *templateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *templateLifecycleDelegate) Remove(obj *Template) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *templateLifecycleDelegate) Updated(obj *Template) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
