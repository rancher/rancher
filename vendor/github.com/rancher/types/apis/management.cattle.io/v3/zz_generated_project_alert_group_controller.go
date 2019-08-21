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
	ProjectAlertGroupGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectAlertGroup",
	}
	ProjectAlertGroupResource = metav1.APIResource{
		Name:         "projectalertgroups",
		SingularName: "projectalertgroup",
		Namespaced:   true,

		Kind: ProjectAlertGroupGroupVersionKind.Kind,
	}

	ProjectAlertGroupGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectalertgroups",
	}
)

func init() {
	resource.Put(ProjectAlertGroupGroupVersionResource)
}

func NewProjectAlertGroup(namespace, name string, obj ProjectAlertGroup) *ProjectAlertGroup {
	obj.APIVersion, obj.Kind = ProjectAlertGroupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectAlertGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectAlertGroup `json:"items"`
}

type ProjectAlertGroupHandlerFunc func(key string, obj *ProjectAlertGroup) (runtime.Object, error)

type ProjectAlertGroupChangeHandlerFunc func(obj *ProjectAlertGroup) (runtime.Object, error)

type ProjectAlertGroupLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectAlertGroup, err error)
	Get(namespace, name string) (*ProjectAlertGroup, error)
}

type ProjectAlertGroupController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectAlertGroupLister
	AddHandler(ctx context.Context, name string, handler ProjectAlertGroupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertGroupHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectAlertGroupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectAlertGroupHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectAlertGroupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectAlertGroup) (*ProjectAlertGroup, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlertGroup, error)
	Get(name string, opts metav1.GetOptions) (*ProjectAlertGroup, error)
	Update(*ProjectAlertGroup) (*ProjectAlertGroup, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectAlertGroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectAlertGroupController
	AddHandler(ctx context.Context, name string, sync ProjectAlertGroupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertGroupHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertGroupLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertGroupLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertGroupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertGroupHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertGroupLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertGroupLifecycle)
}

type projectAlertGroupLister struct {
	controller *projectAlertGroupController
}

func (l *projectAlertGroupLister) List(namespace string, selector labels.Selector) (ret []*ProjectAlertGroup, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectAlertGroup))
	})
	return
}

func (l *projectAlertGroupLister) Get(namespace, name string) (*ProjectAlertGroup, error) {
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
			Group:    ProjectAlertGroupGroupVersionKind.Group,
			Resource: "projectAlertGroup",
		}, key)
	}
	return obj.(*ProjectAlertGroup), nil
}

type projectAlertGroupController struct {
	controller.GenericController
}

func (c *projectAlertGroupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectAlertGroupController) Lister() ProjectAlertGroupLister {
	return &projectAlertGroupLister{
		controller: c,
	}
}

func (c *projectAlertGroupController) AddHandler(ctx context.Context, name string, handler ProjectAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertGroup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertGroupController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertGroup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertGroupController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertGroup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertGroupController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectAlertGroupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertGroup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectAlertGroupFactory struct {
}

func (c projectAlertGroupFactory) Object() runtime.Object {
	return &ProjectAlertGroup{}
}

func (c projectAlertGroupFactory) List() runtime.Object {
	return &ProjectAlertGroupList{}
}

func (s *projectAlertGroupClient) Controller() ProjectAlertGroupController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectAlertGroupControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectAlertGroupGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectAlertGroupController{
		GenericController: genericController,
	}

	s.client.projectAlertGroupControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectAlertGroupClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectAlertGroupController
}

func (s *projectAlertGroupClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectAlertGroupClient) Create(o *ProjectAlertGroup) (*ProjectAlertGroup, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectAlertGroup), err
}

func (s *projectAlertGroupClient) Get(name string, opts metav1.GetOptions) (*ProjectAlertGroup, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectAlertGroup), err
}

func (s *projectAlertGroupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlertGroup, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectAlertGroup), err
}

func (s *projectAlertGroupClient) Update(o *ProjectAlertGroup) (*ProjectAlertGroup, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectAlertGroup), err
}

func (s *projectAlertGroupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectAlertGroupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectAlertGroupClient) List(opts metav1.ListOptions) (*ProjectAlertGroupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectAlertGroupList), err
}

func (s *projectAlertGroupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectAlertGroupClient) Patch(o *ProjectAlertGroup, patchType types.PatchType, data []byte, subresources ...string) (*ProjectAlertGroup, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ProjectAlertGroup), err
}

func (s *projectAlertGroupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectAlertGroupClient) AddHandler(ctx context.Context, name string, sync ProjectAlertGroupHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertGroupClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertGroupHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertGroupClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertGroupLifecycle) {
	sync := NewProjectAlertGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertGroupClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertGroupLifecycle) {
	sync := NewProjectAlertGroupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertGroupClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertGroupHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertGroupClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertGroupHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectAlertGroupClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertGroupLifecycle) {
	sync := NewProjectAlertGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertGroupClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertGroupLifecycle) {
	sync := NewProjectAlertGroupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ProjectAlertGroupIndexer func(obj *ProjectAlertGroup) ([]string, error)

type ProjectAlertGroupClientCache interface {
	Get(namespace, name string) (*ProjectAlertGroup, error)
	List(namespace string, selector labels.Selector) ([]*ProjectAlertGroup, error)

	Index(name string, indexer ProjectAlertGroupIndexer)
	GetIndexed(name, key string) ([]*ProjectAlertGroup, error)
}

type ProjectAlertGroupClient interface {
	Create(*ProjectAlertGroup) (*ProjectAlertGroup, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ProjectAlertGroup, error)
	Update(*ProjectAlertGroup) (*ProjectAlertGroup, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ProjectAlertGroupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ProjectAlertGroupClientCache

	OnCreate(ctx context.Context, name string, sync ProjectAlertGroupChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ProjectAlertGroupChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ProjectAlertGroupChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ProjectAlertGroupInterface
}

type projectAlertGroupClientCache struct {
	client *projectAlertGroupClient2
}

type projectAlertGroupClient2 struct {
	iface      ProjectAlertGroupInterface
	controller ProjectAlertGroupController
}

func (n *projectAlertGroupClient2) Interface() ProjectAlertGroupInterface {
	return n.iface
}

func (n *projectAlertGroupClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *projectAlertGroupClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *projectAlertGroupClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *projectAlertGroupClient2) Create(obj *ProjectAlertGroup) (*ProjectAlertGroup, error) {
	return n.iface.Create(obj)
}

func (n *projectAlertGroupClient2) Get(namespace, name string, opts metav1.GetOptions) (*ProjectAlertGroup, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *projectAlertGroupClient2) Update(obj *ProjectAlertGroup) (*ProjectAlertGroup, error) {
	return n.iface.Update(obj)
}

func (n *projectAlertGroupClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *projectAlertGroupClient2) List(namespace string, opts metav1.ListOptions) (*ProjectAlertGroupList, error) {
	return n.iface.List(opts)
}

func (n *projectAlertGroupClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *projectAlertGroupClientCache) Get(namespace, name string) (*ProjectAlertGroup, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *projectAlertGroupClientCache) List(namespace string, selector labels.Selector) ([]*ProjectAlertGroup, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *projectAlertGroupClient2) Cache() ProjectAlertGroupClientCache {
	n.loadController()
	return &projectAlertGroupClientCache{
		client: n,
	}
}

func (n *projectAlertGroupClient2) OnCreate(ctx context.Context, name string, sync ProjectAlertGroupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &projectAlertGroupLifecycleDelegate{create: sync})
}

func (n *projectAlertGroupClient2) OnChange(ctx context.Context, name string, sync ProjectAlertGroupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &projectAlertGroupLifecycleDelegate{update: sync})
}

func (n *projectAlertGroupClient2) OnRemove(ctx context.Context, name string, sync ProjectAlertGroupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &projectAlertGroupLifecycleDelegate{remove: sync})
}

func (n *projectAlertGroupClientCache) Index(name string, indexer ProjectAlertGroupIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ProjectAlertGroup); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *projectAlertGroupClientCache) GetIndexed(name, key string) ([]*ProjectAlertGroup, error) {
	var result []*ProjectAlertGroup
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ProjectAlertGroup); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *projectAlertGroupClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type projectAlertGroupLifecycleDelegate struct {
	create ProjectAlertGroupChangeHandlerFunc
	update ProjectAlertGroupChangeHandlerFunc
	remove ProjectAlertGroupChangeHandlerFunc
}

func (n *projectAlertGroupLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *projectAlertGroupLifecycleDelegate) Create(obj *ProjectAlertGroup) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *projectAlertGroupLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *projectAlertGroupLifecycleDelegate) Remove(obj *ProjectAlertGroup) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *projectAlertGroupLifecycleDelegate) Updated(obj *ProjectAlertGroup) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
