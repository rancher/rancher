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
	RoleTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RoleTemplate",
	}
	RoleTemplateResource = metav1.APIResource{
		Name:         "roletemplates",
		SingularName: "roletemplate",
		Namespaced:   false,
		Kind:         RoleTemplateGroupVersionKind.Kind,
	}

	RoleTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "roletemplates",
	}
)

func init() {
	resource.Put(RoleTemplateGroupVersionResource)
}

func NewRoleTemplate(namespace, name string, obj RoleTemplate) *RoleTemplate {
	obj.APIVersion, obj.Kind = RoleTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RoleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RoleTemplate `json:"items"`
}

type RoleTemplateHandlerFunc func(key string, obj *RoleTemplate) (runtime.Object, error)

type RoleTemplateChangeHandlerFunc func(obj *RoleTemplate) (runtime.Object, error)

type RoleTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*RoleTemplate, err error)
	Get(namespace, name string) (*RoleTemplate, error)
}

type RoleTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RoleTemplateLister
	AddHandler(ctx context.Context, name string, handler RoleTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RoleTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RoleTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RoleTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*RoleTemplate) (*RoleTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RoleTemplate, error)
	Get(name string, opts metav1.GetOptions) (*RoleTemplate, error)
	Update(*RoleTemplate) (*RoleTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RoleTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RoleTemplateController
	AddHandler(ctx context.Context, name string, sync RoleTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RoleTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleTemplateLifecycle)
}

type roleTemplateLister struct {
	controller *roleTemplateController
}

func (l *roleTemplateLister) List(namespace string, selector labels.Selector) (ret []*RoleTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*RoleTemplate))
	})
	return
}

func (l *roleTemplateLister) Get(namespace, name string) (*RoleTemplate, error) {
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
			Group:    RoleTemplateGroupVersionKind.Group,
			Resource: "roleTemplate",
		}, key)
	}
	return obj.(*RoleTemplate), nil
}

type roleTemplateController struct {
	controller.GenericController
}

func (c *roleTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *roleTemplateController) Lister() RoleTemplateLister {
	return &roleTemplateLister{
		controller: c,
	}
}

func (c *roleTemplateController) AddHandler(ctx context.Context, name string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RoleTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RoleTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RoleTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *roleTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RoleTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RoleTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type roleTemplateFactory struct {
}

func (c roleTemplateFactory) Object() runtime.Object {
	return &RoleTemplate{}
}

func (c roleTemplateFactory) List() runtime.Object {
	return &RoleTemplateList{}
}

func (s *roleTemplateClient) Controller() RoleTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.roleTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RoleTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &roleTemplateController{
		GenericController: genericController,
	}

	s.client.roleTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type roleTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RoleTemplateController
}

func (s *roleTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *roleTemplateClient) Create(o *RoleTemplate) (*RoleTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) Get(name string, opts metav1.GetOptions) (*RoleTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RoleTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) Update(o *RoleTemplate) (*RoleTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *roleTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *roleTemplateClient) List(opts metav1.ListOptions) (*RoleTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RoleTemplateList), err
}

func (s *roleTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *roleTemplateClient) Patch(o *RoleTemplate, patchType types.PatchType, data []byte, subresources ...string) (*RoleTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*RoleTemplate), err
}

func (s *roleTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *roleTemplateClient) AddHandler(ctx context.Context, name string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *roleTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *roleTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RoleTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *roleTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RoleTemplateLifecycle) {
	sync := NewRoleTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type RoleTemplateIndexer func(obj *RoleTemplate) ([]string, error)

type RoleTemplateClientCache interface {
	Get(namespace, name string) (*RoleTemplate, error)
	List(namespace string, selector labels.Selector) ([]*RoleTemplate, error)

	Index(name string, indexer RoleTemplateIndexer)
	GetIndexed(name, key string) ([]*RoleTemplate, error)
}

type RoleTemplateClient interface {
	Create(*RoleTemplate) (*RoleTemplate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*RoleTemplate, error)
	Update(*RoleTemplate) (*RoleTemplate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*RoleTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() RoleTemplateClientCache

	OnCreate(ctx context.Context, name string, sync RoleTemplateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync RoleTemplateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync RoleTemplateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() RoleTemplateInterface
}

type roleTemplateClientCache struct {
	client *roleTemplateClient2
}

type roleTemplateClient2 struct {
	iface      RoleTemplateInterface
	controller RoleTemplateController
}

func (n *roleTemplateClient2) Interface() RoleTemplateInterface {
	return n.iface
}

func (n *roleTemplateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *roleTemplateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *roleTemplateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *roleTemplateClient2) Create(obj *RoleTemplate) (*RoleTemplate, error) {
	return n.iface.Create(obj)
}

func (n *roleTemplateClient2) Get(namespace, name string, opts metav1.GetOptions) (*RoleTemplate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *roleTemplateClient2) Update(obj *RoleTemplate) (*RoleTemplate, error) {
	return n.iface.Update(obj)
}

func (n *roleTemplateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *roleTemplateClient2) List(namespace string, opts metav1.ListOptions) (*RoleTemplateList, error) {
	return n.iface.List(opts)
}

func (n *roleTemplateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *roleTemplateClientCache) Get(namespace, name string) (*RoleTemplate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *roleTemplateClientCache) List(namespace string, selector labels.Selector) ([]*RoleTemplate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *roleTemplateClient2) Cache() RoleTemplateClientCache {
	n.loadController()
	return &roleTemplateClientCache{
		client: n,
	}
}

func (n *roleTemplateClient2) OnCreate(ctx context.Context, name string, sync RoleTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &roleTemplateLifecycleDelegate{create: sync})
}

func (n *roleTemplateClient2) OnChange(ctx context.Context, name string, sync RoleTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &roleTemplateLifecycleDelegate{update: sync})
}

func (n *roleTemplateClient2) OnRemove(ctx context.Context, name string, sync RoleTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &roleTemplateLifecycleDelegate{remove: sync})
}

func (n *roleTemplateClientCache) Index(name string, indexer RoleTemplateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*RoleTemplate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *roleTemplateClientCache) GetIndexed(name, key string) ([]*RoleTemplate, error) {
	var result []*RoleTemplate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*RoleTemplate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *roleTemplateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type roleTemplateLifecycleDelegate struct {
	create RoleTemplateChangeHandlerFunc
	update RoleTemplateChangeHandlerFunc
	remove RoleTemplateChangeHandlerFunc
}

func (n *roleTemplateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *roleTemplateLifecycleDelegate) Create(obj *RoleTemplate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *roleTemplateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *roleTemplateLifecycleDelegate) Remove(obj *RoleTemplate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *roleTemplateLifecycleDelegate) Updated(obj *RoleTemplate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
