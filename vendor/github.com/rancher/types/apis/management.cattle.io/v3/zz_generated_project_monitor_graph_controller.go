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
	ProjectMonitorGraphGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectMonitorGraph",
	}
	ProjectMonitorGraphResource = metav1.APIResource{
		Name:         "projectmonitorgraphs",
		SingularName: "projectmonitorgraph",
		Namespaced:   true,

		Kind: ProjectMonitorGraphGroupVersionKind.Kind,
	}

	ProjectMonitorGraphGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectmonitorgraphs",
	}
)

func init() {
	resource.Put(ProjectMonitorGraphGroupVersionResource)
}

func NewProjectMonitorGraph(namespace, name string, obj ProjectMonitorGraph) *ProjectMonitorGraph {
	obj.APIVersion, obj.Kind = ProjectMonitorGraphGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectMonitorGraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectMonitorGraph `json:"items"`
}

type ProjectMonitorGraphHandlerFunc func(key string, obj *ProjectMonitorGraph) (runtime.Object, error)

type ProjectMonitorGraphChangeHandlerFunc func(obj *ProjectMonitorGraph) (runtime.Object, error)

type ProjectMonitorGraphLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectMonitorGraph, err error)
	Get(namespace, name string) (*ProjectMonitorGraph, error)
}

type ProjectMonitorGraphController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectMonitorGraphLister
	AddHandler(ctx context.Context, name string, handler ProjectMonitorGraphHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectMonitorGraphHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectMonitorGraphHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectMonitorGraphHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectMonitorGraphInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectMonitorGraph) (*ProjectMonitorGraph, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectMonitorGraph, error)
	Get(name string, opts metav1.GetOptions) (*ProjectMonitorGraph, error)
	Update(*ProjectMonitorGraph) (*ProjectMonitorGraph, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectMonitorGraphList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectMonitorGraphController
	AddHandler(ctx context.Context, name string, sync ProjectMonitorGraphHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectMonitorGraphHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectMonitorGraphLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectMonitorGraphLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectMonitorGraphHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectMonitorGraphHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectMonitorGraphLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectMonitorGraphLifecycle)
}

type projectMonitorGraphLister struct {
	controller *projectMonitorGraphController
}

func (l *projectMonitorGraphLister) List(namespace string, selector labels.Selector) (ret []*ProjectMonitorGraph, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectMonitorGraph))
	})
	return
}

func (l *projectMonitorGraphLister) Get(namespace, name string) (*ProjectMonitorGraph, error) {
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
			Group:    ProjectMonitorGraphGroupVersionKind.Group,
			Resource: "projectMonitorGraph",
		}, key)
	}
	return obj.(*ProjectMonitorGraph), nil
}

type projectMonitorGraphController struct {
	controller.GenericController
}

func (c *projectMonitorGraphController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectMonitorGraphController) Lister() ProjectMonitorGraphLister {
	return &projectMonitorGraphLister{
		controller: c,
	}
}

func (c *projectMonitorGraphController) AddHandler(ctx context.Context, name string, handler ProjectMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectMonitorGraph); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectMonitorGraphController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectMonitorGraph); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectMonitorGraphController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectMonitorGraph); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectMonitorGraphController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectMonitorGraphHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectMonitorGraph); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectMonitorGraphFactory struct {
}

func (c projectMonitorGraphFactory) Object() runtime.Object {
	return &ProjectMonitorGraph{}
}

func (c projectMonitorGraphFactory) List() runtime.Object {
	return &ProjectMonitorGraphList{}
}

func (s *projectMonitorGraphClient) Controller() ProjectMonitorGraphController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectMonitorGraphControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectMonitorGraphGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectMonitorGraphController{
		GenericController: genericController,
	}

	s.client.projectMonitorGraphControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectMonitorGraphClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectMonitorGraphController
}

func (s *projectMonitorGraphClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectMonitorGraphClient) Create(o *ProjectMonitorGraph) (*ProjectMonitorGraph, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectMonitorGraph), err
}

func (s *projectMonitorGraphClient) Get(name string, opts metav1.GetOptions) (*ProjectMonitorGraph, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectMonitorGraph), err
}

func (s *projectMonitorGraphClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectMonitorGraph, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectMonitorGraph), err
}

func (s *projectMonitorGraphClient) Update(o *ProjectMonitorGraph) (*ProjectMonitorGraph, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectMonitorGraph), err
}

func (s *projectMonitorGraphClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectMonitorGraphClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectMonitorGraphClient) List(opts metav1.ListOptions) (*ProjectMonitorGraphList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectMonitorGraphList), err
}

func (s *projectMonitorGraphClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectMonitorGraphClient) Patch(o *ProjectMonitorGraph, patchType types.PatchType, data []byte, subresources ...string) (*ProjectMonitorGraph, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ProjectMonitorGraph), err
}

func (s *projectMonitorGraphClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectMonitorGraphClient) AddHandler(ctx context.Context, name string, sync ProjectMonitorGraphHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectMonitorGraphClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectMonitorGraphHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectMonitorGraphClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectMonitorGraphLifecycle) {
	sync := NewProjectMonitorGraphLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectMonitorGraphClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectMonitorGraphLifecycle) {
	sync := NewProjectMonitorGraphLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectMonitorGraphClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectMonitorGraphHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectMonitorGraphClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectMonitorGraphHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectMonitorGraphClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectMonitorGraphLifecycle) {
	sync := NewProjectMonitorGraphLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectMonitorGraphClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectMonitorGraphLifecycle) {
	sync := NewProjectMonitorGraphLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ProjectMonitorGraphIndexer func(obj *ProjectMonitorGraph) ([]string, error)

type ProjectMonitorGraphClientCache interface {
	Get(namespace, name string) (*ProjectMonitorGraph, error)
	List(namespace string, selector labels.Selector) ([]*ProjectMonitorGraph, error)

	Index(name string, indexer ProjectMonitorGraphIndexer)
	GetIndexed(name, key string) ([]*ProjectMonitorGraph, error)
}

type ProjectMonitorGraphClient interface {
	Create(*ProjectMonitorGraph) (*ProjectMonitorGraph, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ProjectMonitorGraph, error)
	Update(*ProjectMonitorGraph) (*ProjectMonitorGraph, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ProjectMonitorGraphList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ProjectMonitorGraphClientCache

	OnCreate(ctx context.Context, name string, sync ProjectMonitorGraphChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ProjectMonitorGraphChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ProjectMonitorGraphChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ProjectMonitorGraphInterface
}

type projectMonitorGraphClientCache struct {
	client *projectMonitorGraphClient2
}

type projectMonitorGraphClient2 struct {
	iface      ProjectMonitorGraphInterface
	controller ProjectMonitorGraphController
}

func (n *projectMonitorGraphClient2) Interface() ProjectMonitorGraphInterface {
	return n.iface
}

func (n *projectMonitorGraphClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *projectMonitorGraphClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *projectMonitorGraphClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *projectMonitorGraphClient2) Create(obj *ProjectMonitorGraph) (*ProjectMonitorGraph, error) {
	return n.iface.Create(obj)
}

func (n *projectMonitorGraphClient2) Get(namespace, name string, opts metav1.GetOptions) (*ProjectMonitorGraph, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *projectMonitorGraphClient2) Update(obj *ProjectMonitorGraph) (*ProjectMonitorGraph, error) {
	return n.iface.Update(obj)
}

func (n *projectMonitorGraphClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *projectMonitorGraphClient2) List(namespace string, opts metav1.ListOptions) (*ProjectMonitorGraphList, error) {
	return n.iface.List(opts)
}

func (n *projectMonitorGraphClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *projectMonitorGraphClientCache) Get(namespace, name string) (*ProjectMonitorGraph, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *projectMonitorGraphClientCache) List(namespace string, selector labels.Selector) ([]*ProjectMonitorGraph, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *projectMonitorGraphClient2) Cache() ProjectMonitorGraphClientCache {
	n.loadController()
	return &projectMonitorGraphClientCache{
		client: n,
	}
}

func (n *projectMonitorGraphClient2) OnCreate(ctx context.Context, name string, sync ProjectMonitorGraphChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &projectMonitorGraphLifecycleDelegate{create: sync})
}

func (n *projectMonitorGraphClient2) OnChange(ctx context.Context, name string, sync ProjectMonitorGraphChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &projectMonitorGraphLifecycleDelegate{update: sync})
}

func (n *projectMonitorGraphClient2) OnRemove(ctx context.Context, name string, sync ProjectMonitorGraphChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &projectMonitorGraphLifecycleDelegate{remove: sync})
}

func (n *projectMonitorGraphClientCache) Index(name string, indexer ProjectMonitorGraphIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ProjectMonitorGraph); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *projectMonitorGraphClientCache) GetIndexed(name, key string) ([]*ProjectMonitorGraph, error) {
	var result []*ProjectMonitorGraph
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ProjectMonitorGraph); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *projectMonitorGraphClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type projectMonitorGraphLifecycleDelegate struct {
	create ProjectMonitorGraphChangeHandlerFunc
	update ProjectMonitorGraphChangeHandlerFunc
	remove ProjectMonitorGraphChangeHandlerFunc
}

func (n *projectMonitorGraphLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *projectMonitorGraphLifecycleDelegate) Create(obj *ProjectMonitorGraph) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *projectMonitorGraphLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *projectMonitorGraphLifecycleDelegate) Remove(obj *ProjectMonitorGraph) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *projectMonitorGraphLifecycleDelegate) Updated(obj *ProjectMonitorGraph) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
