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
	NodeTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NodeTemplate",
	}
	NodeTemplateResource = metav1.APIResource{
		Name:         "nodetemplates",
		SingularName: "nodetemplate",
		Namespaced:   true,

		Kind: NodeTemplateGroupVersionKind.Kind,
	}

	NodeTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "nodetemplates",
	}
)

func init() {
	resource.Put(NodeTemplateGroupVersionResource)
}

func NewNodeTemplate(namespace, name string, obj NodeTemplate) *NodeTemplate {
	obj.APIVersion, obj.Kind = NodeTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type NodeTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeTemplate `json:"items"`
}

type NodeTemplateHandlerFunc func(key string, obj *NodeTemplate) (runtime.Object, error)

type NodeTemplateChangeHandlerFunc func(obj *NodeTemplate) (runtime.Object, error)

type NodeTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*NodeTemplate, err error)
	Get(namespace, name string) (*NodeTemplate, error)
}

type NodeTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() NodeTemplateLister
	AddHandler(ctx context.Context, name string, handler NodeTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler NodeTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler NodeTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NodeTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*NodeTemplate) (*NodeTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodeTemplate, error)
	Get(name string, opts metav1.GetOptions) (*NodeTemplate, error)
	Update(*NodeTemplate) (*NodeTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NodeTemplateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*NodeTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NodeTemplateController
	AddHandler(ctx context.Context, name string, sync NodeTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle NodeTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeTemplateLifecycle)
}

type nodeTemplateLister struct {
	controller *nodeTemplateController
}

func (l *nodeTemplateLister) List(namespace string, selector labels.Selector) (ret []*NodeTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NodeTemplate))
	})
	return
}

func (l *nodeTemplateLister) Get(namespace, name string) (*NodeTemplate, error) {
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
			Group:    NodeTemplateGroupVersionKind.Group,
			Resource: "nodeTemplate",
		}, key)
	}
	return obj.(*NodeTemplate), nil
}

type nodeTemplateController struct {
	controller.GenericController
}

func (c *nodeTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *nodeTemplateController) Lister() NodeTemplateLister {
	return &nodeTemplateLister{
		controller: c,
	}
}

func (c *nodeTemplateController) AddHandler(ctx context.Context, name string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *nodeTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler NodeTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*NodeTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type nodeTemplateFactory struct {
}

func (c nodeTemplateFactory) Object() runtime.Object {
	return &NodeTemplate{}
}

func (c nodeTemplateFactory) List() runtime.Object {
	return &NodeTemplateList{}
}

func (s *nodeTemplateClient) Controller() NodeTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.nodeTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NodeTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &nodeTemplateController{
		GenericController: genericController,
	}

	s.client.nodeTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type nodeTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   NodeTemplateController
}

func (s *nodeTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *nodeTemplateClient) Create(o *NodeTemplate) (*NodeTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NodeTemplate), err
}

func (s *nodeTemplateClient) Get(name string, opts metav1.GetOptions) (*NodeTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NodeTemplate), err
}

func (s *nodeTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*NodeTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*NodeTemplate), err
}

func (s *nodeTemplateClient) Update(o *NodeTemplate) (*NodeTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NodeTemplate), err
}

func (s *nodeTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *nodeTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *nodeTemplateClient) List(opts metav1.ListOptions) (*NodeTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NodeTemplateList), err
}

func (s *nodeTemplateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*NodeTemplateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*NodeTemplateList), err
}

func (s *nodeTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *nodeTemplateClient) Patch(o *NodeTemplate, patchType types.PatchType, data []byte, subresources ...string) (*NodeTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*NodeTemplate), err
}

func (s *nodeTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *nodeTemplateClient) AddHandler(ctx context.Context, name string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *nodeTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *nodeTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync NodeTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *nodeTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *nodeTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle NodeTemplateLifecycle) {
	sync := NewNodeTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type NodeTemplateIndexer func(obj *NodeTemplate) ([]string, error)

type NodeTemplateClientCache interface {
	Get(namespace, name string) (*NodeTemplate, error)
	List(namespace string, selector labels.Selector) ([]*NodeTemplate, error)

	Index(name string, indexer NodeTemplateIndexer)
	GetIndexed(name, key string) ([]*NodeTemplate, error)
}

type NodeTemplateClient interface {
	Create(*NodeTemplate) (*NodeTemplate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*NodeTemplate, error)
	Update(*NodeTemplate) (*NodeTemplate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*NodeTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() NodeTemplateClientCache

	OnCreate(ctx context.Context, name string, sync NodeTemplateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync NodeTemplateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync NodeTemplateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() NodeTemplateInterface
}

type nodeTemplateClientCache struct {
	client *nodeTemplateClient2
}

type nodeTemplateClient2 struct {
	iface      NodeTemplateInterface
	controller NodeTemplateController
}

func (n *nodeTemplateClient2) Interface() NodeTemplateInterface {
	return n.iface
}

func (n *nodeTemplateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *nodeTemplateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *nodeTemplateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *nodeTemplateClient2) Create(obj *NodeTemplate) (*NodeTemplate, error) {
	return n.iface.Create(obj)
}

func (n *nodeTemplateClient2) Get(namespace, name string, opts metav1.GetOptions) (*NodeTemplate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *nodeTemplateClient2) Update(obj *NodeTemplate) (*NodeTemplate, error) {
	return n.iface.Update(obj)
}

func (n *nodeTemplateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *nodeTemplateClient2) List(namespace string, opts metav1.ListOptions) (*NodeTemplateList, error) {
	return n.iface.List(opts)
}

func (n *nodeTemplateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *nodeTemplateClientCache) Get(namespace, name string) (*NodeTemplate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *nodeTemplateClientCache) List(namespace string, selector labels.Selector) ([]*NodeTemplate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *nodeTemplateClient2) Cache() NodeTemplateClientCache {
	n.loadController()
	return &nodeTemplateClientCache{
		client: n,
	}
}

func (n *nodeTemplateClient2) OnCreate(ctx context.Context, name string, sync NodeTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &nodeTemplateLifecycleDelegate{create: sync})
}

func (n *nodeTemplateClient2) OnChange(ctx context.Context, name string, sync NodeTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &nodeTemplateLifecycleDelegate{update: sync})
}

func (n *nodeTemplateClient2) OnRemove(ctx context.Context, name string, sync NodeTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &nodeTemplateLifecycleDelegate{remove: sync})
}

func (n *nodeTemplateClientCache) Index(name string, indexer NodeTemplateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*NodeTemplate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *nodeTemplateClientCache) GetIndexed(name, key string) ([]*NodeTemplate, error) {
	var result []*NodeTemplate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*NodeTemplate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *nodeTemplateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type nodeTemplateLifecycleDelegate struct {
	create NodeTemplateChangeHandlerFunc
	update NodeTemplateChangeHandlerFunc
	remove NodeTemplateChangeHandlerFunc
}

func (n *nodeTemplateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *nodeTemplateLifecycleDelegate) Create(obj *NodeTemplate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *nodeTemplateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *nodeTemplateLifecycleDelegate) Remove(obj *NodeTemplate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *nodeTemplateLifecycleDelegate) Updated(obj *NodeTemplate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
