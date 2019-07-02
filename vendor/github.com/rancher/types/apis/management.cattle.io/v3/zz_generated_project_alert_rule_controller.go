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
	ProjectAlertRuleGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ProjectAlertRule",
	}
	ProjectAlertRuleResource = metav1.APIResource{
		Name:         "projectalertrules",
		SingularName: "projectalertrule",
		Namespaced:   true,

		Kind: ProjectAlertRuleGroupVersionKind.Kind,
	}

	ProjectAlertRuleGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "projectalertrules",
	}
)

func init() {
	resource.Put(ProjectAlertRuleGroupVersionResource)
}

func NewProjectAlertRule(namespace, name string, obj ProjectAlertRule) *ProjectAlertRule {
	obj.APIVersion, obj.Kind = ProjectAlertRuleGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ProjectAlertRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectAlertRule `json:"items"`
}

type ProjectAlertRuleHandlerFunc func(key string, obj *ProjectAlertRule) (runtime.Object, error)

type ProjectAlertRuleChangeHandlerFunc func(obj *ProjectAlertRule) (runtime.Object, error)

type ProjectAlertRuleLister interface {
	List(namespace string, selector labels.Selector) (ret []*ProjectAlertRule, err error)
	Get(namespace, name string) (*ProjectAlertRule, error)
}

type ProjectAlertRuleController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ProjectAlertRuleLister
	AddHandler(ctx context.Context, name string, handler ProjectAlertRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertRuleHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ProjectAlertRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ProjectAlertRuleHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ProjectAlertRuleInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ProjectAlertRule) (*ProjectAlertRule, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlertRule, error)
	Get(name string, opts metav1.GetOptions) (*ProjectAlertRule, error)
	Update(*ProjectAlertRule) (*ProjectAlertRule, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ProjectAlertRuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ProjectAlertRuleController
	AddHandler(ctx context.Context, name string, sync ProjectAlertRuleHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertRuleHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertRuleLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertRuleLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertRuleHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertRuleHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertRuleLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertRuleLifecycle)
}

type projectAlertRuleLister struct {
	controller *projectAlertRuleController
}

func (l *projectAlertRuleLister) List(namespace string, selector labels.Selector) (ret []*ProjectAlertRule, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ProjectAlertRule))
	})
	return
}

func (l *projectAlertRuleLister) Get(namespace, name string) (*ProjectAlertRule, error) {
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
			Group:    ProjectAlertRuleGroupVersionKind.Group,
			Resource: "projectAlertRule",
		}, key)
	}
	return obj.(*ProjectAlertRule), nil
}

type projectAlertRuleController struct {
	controller.GenericController
}

func (c *projectAlertRuleController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *projectAlertRuleController) Lister() ProjectAlertRuleLister {
	return &projectAlertRuleLister{
		controller: c,
	}
}

func (c *projectAlertRuleController) AddHandler(ctx context.Context, name string, handler ProjectAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertRuleController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ProjectAlertRuleHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertRule); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertRuleController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ProjectAlertRuleHandlerFunc) {
	resource.PutClusterScoped(ProjectAlertRuleGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *projectAlertRuleController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ProjectAlertRuleHandlerFunc) {
	resource.PutClusterScoped(ProjectAlertRuleGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ProjectAlertRule); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type projectAlertRuleFactory struct {
}

func (c projectAlertRuleFactory) Object() runtime.Object {
	return &ProjectAlertRule{}
}

func (c projectAlertRuleFactory) List() runtime.Object {
	return &ProjectAlertRuleList{}
}

func (s *projectAlertRuleClient) Controller() ProjectAlertRuleController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.projectAlertRuleControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ProjectAlertRuleGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &projectAlertRuleController{
		GenericController: genericController,
	}

	s.client.projectAlertRuleControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type projectAlertRuleClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ProjectAlertRuleController
}

func (s *projectAlertRuleClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *projectAlertRuleClient) Create(o *ProjectAlertRule) (*ProjectAlertRule, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ProjectAlertRule), err
}

func (s *projectAlertRuleClient) Get(name string, opts metav1.GetOptions) (*ProjectAlertRule, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ProjectAlertRule), err
}

func (s *projectAlertRuleClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ProjectAlertRule, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ProjectAlertRule), err
}

func (s *projectAlertRuleClient) Update(o *ProjectAlertRule) (*ProjectAlertRule, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ProjectAlertRule), err
}

func (s *projectAlertRuleClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *projectAlertRuleClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *projectAlertRuleClient) List(opts metav1.ListOptions) (*ProjectAlertRuleList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ProjectAlertRuleList), err
}

func (s *projectAlertRuleClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *projectAlertRuleClient) Patch(o *ProjectAlertRule, patchType types.PatchType, data []byte, subresources ...string) (*ProjectAlertRule, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ProjectAlertRule), err
}

func (s *projectAlertRuleClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *projectAlertRuleClient) AddHandler(ctx context.Context, name string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertRuleClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertRuleClient) AddLifecycle(ctx context.Context, name string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *projectAlertRuleClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ProjectAlertRuleHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *projectAlertRuleClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ProjectAlertRuleLifecycle) {
	sync := NewProjectAlertRuleLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ProjectAlertRuleIndexer func(obj *ProjectAlertRule) ([]string, error)

type ProjectAlertRuleClientCache interface {
	Get(namespace, name string) (*ProjectAlertRule, error)
	List(namespace string, selector labels.Selector) ([]*ProjectAlertRule, error)

	Index(name string, indexer ProjectAlertRuleIndexer)
	GetIndexed(name, key string) ([]*ProjectAlertRule, error)
}

type ProjectAlertRuleClient interface {
	Create(*ProjectAlertRule) (*ProjectAlertRule, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ProjectAlertRule, error)
	Update(*ProjectAlertRule) (*ProjectAlertRule, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ProjectAlertRuleList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ProjectAlertRuleClientCache

	OnCreate(ctx context.Context, name string, sync ProjectAlertRuleChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ProjectAlertRuleChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ProjectAlertRuleChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ProjectAlertRuleInterface
}

type projectAlertRuleClientCache struct {
	client *projectAlertRuleClient2
}

type projectAlertRuleClient2 struct {
	iface      ProjectAlertRuleInterface
	controller ProjectAlertRuleController
}

func (n *projectAlertRuleClient2) Interface() ProjectAlertRuleInterface {
	return n.iface
}

func (n *projectAlertRuleClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *projectAlertRuleClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *projectAlertRuleClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *projectAlertRuleClient2) Create(obj *ProjectAlertRule) (*ProjectAlertRule, error) {
	return n.iface.Create(obj)
}

func (n *projectAlertRuleClient2) Get(namespace, name string, opts metav1.GetOptions) (*ProjectAlertRule, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *projectAlertRuleClient2) Update(obj *ProjectAlertRule) (*ProjectAlertRule, error) {
	return n.iface.Update(obj)
}

func (n *projectAlertRuleClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *projectAlertRuleClient2) List(namespace string, opts metav1.ListOptions) (*ProjectAlertRuleList, error) {
	return n.iface.List(opts)
}

func (n *projectAlertRuleClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *projectAlertRuleClientCache) Get(namespace, name string) (*ProjectAlertRule, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *projectAlertRuleClientCache) List(namespace string, selector labels.Selector) ([]*ProjectAlertRule, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *projectAlertRuleClient2) Cache() ProjectAlertRuleClientCache {
	n.loadController()
	return &projectAlertRuleClientCache{
		client: n,
	}
}

func (n *projectAlertRuleClient2) OnCreate(ctx context.Context, name string, sync ProjectAlertRuleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &projectAlertRuleLifecycleDelegate{create: sync})
}

func (n *projectAlertRuleClient2) OnChange(ctx context.Context, name string, sync ProjectAlertRuleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &projectAlertRuleLifecycleDelegate{update: sync})
}

func (n *projectAlertRuleClient2) OnRemove(ctx context.Context, name string, sync ProjectAlertRuleChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &projectAlertRuleLifecycleDelegate{remove: sync})
}

func (n *projectAlertRuleClientCache) Index(name string, indexer ProjectAlertRuleIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ProjectAlertRule); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *projectAlertRuleClientCache) GetIndexed(name, key string) ([]*ProjectAlertRule, error) {
	var result []*ProjectAlertRule
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ProjectAlertRule); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *projectAlertRuleClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type projectAlertRuleLifecycleDelegate struct {
	create ProjectAlertRuleChangeHandlerFunc
	update ProjectAlertRuleChangeHandlerFunc
	remove ProjectAlertRuleChangeHandlerFunc
}

func (n *projectAlertRuleLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *projectAlertRuleLifecycleDelegate) Create(obj *ProjectAlertRule) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *projectAlertRuleLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *projectAlertRuleLifecycleDelegate) Remove(obj *ProjectAlertRule) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *projectAlertRuleLifecycleDelegate) Updated(obj *ProjectAlertRule) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
