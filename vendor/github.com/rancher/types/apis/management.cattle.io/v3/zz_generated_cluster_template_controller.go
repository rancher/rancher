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
	ClusterTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterTemplate",
	}
	ClusterTemplateResource = metav1.APIResource{
		Name:         "clustertemplates",
		SingularName: "clustertemplate",
		Namespaced:   true,

		Kind: ClusterTemplateGroupVersionKind.Kind,
	}

	ClusterTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clustertemplates",
	}
)

func init() {
	resource.Put(ClusterTemplateGroupVersionResource)
}

func NewClusterTemplate(namespace, name string, obj ClusterTemplate) *ClusterTemplate {
	obj.APIVersion, obj.Kind = ClusterTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTemplate `json:"items"`
}

type ClusterTemplateHandlerFunc func(key string, obj *ClusterTemplate) (runtime.Object, error)

type ClusterTemplateChangeHandlerFunc func(obj *ClusterTemplate) (runtime.Object, error)

type ClusterTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterTemplate, err error)
	Get(namespace, name string) (*ClusterTemplate, error)
}

type ClusterTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterTemplateLister
	AddHandler(ctx context.Context, name string, handler ClusterTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterTemplate) (*ClusterTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterTemplate, error)
	Get(name string, opts metav1.GetOptions) (*ClusterTemplate, error)
	Update(*ClusterTemplate) (*ClusterTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterTemplateController
	AddHandler(ctx context.Context, name string, sync ClusterTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterTemplateLifecycle)
}

type clusterTemplateLister struct {
	controller *clusterTemplateController
}

func (l *clusterTemplateLister) List(namespace string, selector labels.Selector) (ret []*ClusterTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterTemplate))
	})
	return
}

func (l *clusterTemplateLister) Get(namespace, name string) (*ClusterTemplate, error) {
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
			Group:    ClusterTemplateGroupVersionKind.Group,
			Resource: "clusterTemplate",
		}, key)
	}
	return obj.(*ClusterTemplate), nil
}

type clusterTemplateController struct {
	controller.GenericController
}

func (c *clusterTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterTemplateController) Lister() ClusterTemplateLister {
	return &clusterTemplateLister{
		controller: c,
	}
}

func (c *clusterTemplateController) AddHandler(ctx context.Context, name string, handler ClusterTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterTemplateHandlerFunc) {
	resource.PutClusterScoped(ClusterTemplateGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterTemplateHandlerFunc) {
	resource.PutClusterScoped(ClusterTemplateGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterTemplateFactory struct {
}

func (c clusterTemplateFactory) Object() runtime.Object {
	return &ClusterTemplate{}
}

func (c clusterTemplateFactory) List() runtime.Object {
	return &ClusterTemplateList{}
}

func (s *clusterTemplateClient) Controller() ClusterTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterTemplateController{
		GenericController: genericController,
	}

	s.client.clusterTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterTemplateController
}

func (s *clusterTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterTemplateClient) Create(o *ClusterTemplate) (*ClusterTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterTemplate), err
}

func (s *clusterTemplateClient) Get(name string, opts metav1.GetOptions) (*ClusterTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterTemplate), err
}

func (s *clusterTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterTemplate), err
}

func (s *clusterTemplateClient) Update(o *ClusterTemplate) (*ClusterTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterTemplate), err
}

func (s *clusterTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterTemplateClient) List(opts metav1.ListOptions) (*ClusterTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterTemplateList), err
}

func (s *clusterTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterTemplateClient) Patch(o *ClusterTemplate, patchType types.PatchType, data []byte, subresources ...string) (*ClusterTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterTemplate), err
}

func (s *clusterTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterTemplateClient) AddHandler(ctx context.Context, name string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterTemplateLifecycle) {
	sync := NewClusterTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ClusterTemplateIndexer func(obj *ClusterTemplate) ([]string, error)

type ClusterTemplateClientCache interface {
	Get(namespace, name string) (*ClusterTemplate, error)
	List(namespace string, selector labels.Selector) ([]*ClusterTemplate, error)

	Index(name string, indexer ClusterTemplateIndexer)
	GetIndexed(name, key string) ([]*ClusterTemplate, error)
}

type ClusterTemplateClient interface {
	Create(*ClusterTemplate) (*ClusterTemplate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ClusterTemplate, error)
	Update(*ClusterTemplate) (*ClusterTemplate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterTemplateClientCache

	OnCreate(ctx context.Context, name string, sync ClusterTemplateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterTemplateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterTemplateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterTemplateInterface
}

type clusterTemplateClientCache struct {
	client *clusterTemplateClient2
}

type clusterTemplateClient2 struct {
	iface      ClusterTemplateInterface
	controller ClusterTemplateController
}

func (n *clusterTemplateClient2) Interface() ClusterTemplateInterface {
	return n.iface
}

func (n *clusterTemplateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterTemplateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterTemplateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterTemplateClient2) Create(obj *ClusterTemplate) (*ClusterTemplate, error) {
	return n.iface.Create(obj)
}

func (n *clusterTemplateClient2) Get(namespace, name string, opts metav1.GetOptions) (*ClusterTemplate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterTemplateClient2) Update(obj *ClusterTemplate) (*ClusterTemplate, error) {
	return n.iface.Update(obj)
}

func (n *clusterTemplateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterTemplateClient2) List(namespace string, opts metav1.ListOptions) (*ClusterTemplateList, error) {
	return n.iface.List(opts)
}

func (n *clusterTemplateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterTemplateClientCache) Get(namespace, name string) (*ClusterTemplate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterTemplateClientCache) List(namespace string, selector labels.Selector) ([]*ClusterTemplate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterTemplateClient2) Cache() ClusterTemplateClientCache {
	n.loadController()
	return &clusterTemplateClientCache{
		client: n,
	}
}

func (n *clusterTemplateClient2) OnCreate(ctx context.Context, name string, sync ClusterTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterTemplateLifecycleDelegate{create: sync})
}

func (n *clusterTemplateClient2) OnChange(ctx context.Context, name string, sync ClusterTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterTemplateLifecycleDelegate{update: sync})
}

func (n *clusterTemplateClient2) OnRemove(ctx context.Context, name string, sync ClusterTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterTemplateLifecycleDelegate{remove: sync})
}

func (n *clusterTemplateClientCache) Index(name string, indexer ClusterTemplateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ClusterTemplate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterTemplateClientCache) GetIndexed(name, key string) ([]*ClusterTemplate, error) {
	var result []*ClusterTemplate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ClusterTemplate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterTemplateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterTemplateLifecycleDelegate struct {
	create ClusterTemplateChangeHandlerFunc
	update ClusterTemplateChangeHandlerFunc
	remove ClusterTemplateChangeHandlerFunc
}

func (n *clusterTemplateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterTemplateLifecycleDelegate) Create(obj *ClusterTemplate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterTemplateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterTemplateLifecycleDelegate) Remove(obj *ClusterTemplate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterTemplateLifecycleDelegate) Updated(obj *ClusterTemplate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
