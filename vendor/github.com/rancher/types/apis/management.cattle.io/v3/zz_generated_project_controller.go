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
	ProjectGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Project",
	}
	ProjectResource = metav1.APIResource{
		Name:         "projects",
		SingularName: "project",
		Namespaced:   true,

		Kind: ProjectGroupVersionKind.Kind,
	}

	ProjectGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projects",
	}
)

func init() {
	resource.Put(ProjectGroupVersionResource)
}

func NewProject(namespace, name string, obj Project) *Project {
	obj.APIVersion, obj.Kind = ProjectGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

type ProjectHandlerFunc func(key string, obj *Project) (runtime.Object, error)

type ProjectChangeHandlerFunc func(obj *Project) (runtime.Object, error)

type ProjectLister interface {
	List(namespace string, selector labels.Selector) (ret []*Project, err error)
	Get(namespace, name string) (*Project, error)
}

type ProjectController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectLister
	AddHandler(ctx context.Context, name string, handler ProjectHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Project) (*Project, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Project, error)
	Get(name string, opts metav1.GetOptions) (*Project, error)
	Update(*Project) (*Project, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectController
	AddHandler(ctx context.Context, name string, sync ProjectHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectLifecycle)
}

type projectLister struct {
	controller *projectController
}

func (l *projectLister) List(namespace string, selector labels.Selector) (ret []*Project, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Project))
	})
	return
}

func (l *projectLister) Get(namespace, name string) (*Project, error) {
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
			Group:    ProjectGroupVersionKind.Group,
			Resource: "project",
		}, key)
	}
	return obj.(*Project), nil
}

type projectController struct {
	controller.GenericController
}

func (c *projectController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectController) Lister() ProjectLister {
	return &projectLister{
		controller: c,
	}
}

func (c *projectController) AddHandler(ctx context.Context, name string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Project); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Project); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Project); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Project); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectFactory struct {
}

func (c projectFactory) Object() runtime.Object {
	return &Project{}
}

func (c projectFactory) List() runtime.Object {
	return &ProjectList{}
}

func (s *projectClient) Controller() ProjectController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectController{
		GenericController: genericController,
	}

	s.client.projectControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectController
}

func (s *projectClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectClient) Create(o *Project) (*Project, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Project), err
}

func (s *projectClient) Get(name string, opts metav1.GetOptions) (*Project, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Project), err
}

func (s *projectClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Project, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Project), err
}

func (s *projectClient) Update(o *Project) (*Project, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Project), err
}

func (s *projectClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectClient) List(opts metav1.ListOptions) (*ProjectList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectList), err
}

func (s *projectClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectClient) Patch(o *Project, patchType types.PatchType, data []byte, subresources ...string) (*Project, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Project), err
}

func (s *projectClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectClient) AddHandler(ctx context.Context, name string, sync ProjectHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectLifecycle) {
	sync := NewProjectLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ProjectIndexer func(obj *Project) ([]string, error)

type ProjectClientCache interface {
	Get(namespace, name string) (*Project, error)
	List(namespace string, selector labels.Selector) ([]*Project, error)

	Index(name string, indexer ProjectIndexer)
	GetIndexed(name, key string) ([]*Project, error)
}

type ProjectClient interface {
	Create(*Project) (*Project, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Project, error)
	Update(*Project) (*Project, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ProjectList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ProjectClientCache

	OnCreate(ctx context.Context, name string, sync ProjectChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ProjectChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ProjectChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ProjectInterface
}

type projectClientCache struct {
	client *projectClient2
}

type projectClient2 struct {
	iface      ProjectInterface
	controller ProjectController
}

func (n *projectClient2) Interface() ProjectInterface {
	return n.iface
}

func (n *projectClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *projectClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *projectClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *projectClient2) Create(obj *Project) (*Project, error) {
	return n.iface.Create(obj)
}

func (n *projectClient2) Get(namespace, name string, opts metav1.GetOptions) (*Project, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *projectClient2) Update(obj *Project) (*Project, error) {
	return n.iface.Update(obj)
}

func (n *projectClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *projectClient2) List(namespace string, opts metav1.ListOptions) (*ProjectList, error) {
	return n.iface.List(opts)
}

func (n *projectClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *projectClientCache) Get(namespace, name string) (*Project, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *projectClientCache) List(namespace string, selector labels.Selector) ([]*Project, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *projectClient2) Cache() ProjectClientCache {
	n.loadController()
	return &projectClientCache{
		client: n,
	}
}

func (n *projectClient2) OnCreate(ctx context.Context, name string, sync ProjectChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &projectLifecycleDelegate{create: sync})
}

func (n *projectClient2) OnChange(ctx context.Context, name string, sync ProjectChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &projectLifecycleDelegate{update: sync})
}

func (n *projectClient2) OnRemove(ctx context.Context, name string, sync ProjectChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &projectLifecycleDelegate{remove: sync})
}

func (n *projectClientCache) Index(name string, indexer ProjectIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Project); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *projectClientCache) GetIndexed(name, key string) ([]*Project, error) {
	var result []*Project
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Project); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *projectClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type projectLifecycleDelegate struct {
	create ProjectChangeHandlerFunc
	update ProjectChangeHandlerFunc
	remove ProjectChangeHandlerFunc
}

func (n *projectLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *projectLifecycleDelegate) Create(obj *Project) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *projectLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *projectLifecycleDelegate) Remove(obj *Project) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *projectLifecycleDelegate) Updated(obj *Project) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
