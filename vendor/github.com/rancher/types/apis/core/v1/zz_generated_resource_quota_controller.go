package v1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	v1 "k8s.io/api/core/v1"
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
	ResourceQuotaGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ResourceQuota",
	}
	ResourceQuotaResource = metav1.APIResource{
		Name:         "resourcequotas",
		SingularName: "resourcequota",
		Namespaced:   true,

		Kind: ResourceQuotaGroupVersionKind.Kind,
	}

	ResourceQuotaGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "resourcequotas",
	}
)

func init() {
	resource.Put(ResourceQuotaGroupVersionResource)
}

func NewResourceQuota(namespace, name string, obj v1.ResourceQuota) *v1.ResourceQuota {
	obj.APIVersion, obj.Kind = ResourceQuotaGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ResourceQuota `json:"items"`
}

type ResourceQuotaHandlerFunc func(key string, obj *v1.ResourceQuota) (runtime.Object, error)

type ResourceQuotaChangeHandlerFunc func(obj *v1.ResourceQuota) (runtime.Object, error)

type ResourceQuotaLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ResourceQuota, err error)
	Get(namespace, name string) (*v1.ResourceQuota, error)
}

type ResourceQuotaController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ResourceQuotaLister
	AddHandler(ctx context.Context, name string, handler ResourceQuotaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ResourceQuotaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ResourceQuotaHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ResourceQuotaInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Get(name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Update(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ResourceQuotaList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ResourceQuotaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ResourceQuotaController
	AddHandler(ctx context.Context, name string, sync ResourceQuotaHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ResourceQuotaHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ResourceQuotaLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ResourceQuotaLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ResourceQuotaHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ResourceQuotaLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ResourceQuotaLifecycle)
}

type resourceQuotaLister struct {
	controller *resourceQuotaController
}

func (l *resourceQuotaLister) List(namespace string, selector labels.Selector) (ret []*v1.ResourceQuota, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ResourceQuota))
	})
	return
}

func (l *resourceQuotaLister) Get(namespace, name string) (*v1.ResourceQuota, error) {
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
			Group:    ResourceQuotaGroupVersionKind.Group,
			Resource: "resourceQuota",
		}, key)
	}
	return obj.(*v1.ResourceQuota), nil
}

type resourceQuotaController struct {
	controller.GenericController
}

func (c *resourceQuotaController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *resourceQuotaController) Lister() ResourceQuotaLister {
	return &resourceQuotaLister{
		controller: c,
	}
}

func (c *resourceQuotaController) AddHandler(ctx context.Context, name string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *resourceQuotaController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *resourceQuotaController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *resourceQuotaController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ResourceQuotaHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ResourceQuota); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type resourceQuotaFactory struct {
}

func (c resourceQuotaFactory) Object() runtime.Object {
	return &v1.ResourceQuota{}
}

func (c resourceQuotaFactory) List() runtime.Object {
	return &ResourceQuotaList{}
}

func (s *resourceQuotaClient) Controller() ResourceQuotaController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.resourceQuotaControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ResourceQuotaGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &resourceQuotaController{
		GenericController: genericController,
	}

	s.client.resourceQuotaControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type resourceQuotaClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ResourceQuotaController
}

func (s *resourceQuotaClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *resourceQuotaClient) Create(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Get(name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Update(o *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *resourceQuotaClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *resourceQuotaClient) List(opts metav1.ListOptions) (*ResourceQuotaList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ResourceQuotaList), err
}

func (s *resourceQuotaClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ResourceQuotaList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ResourceQuotaList), err
}

func (s *resourceQuotaClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *resourceQuotaClient) Patch(o *v1.ResourceQuota, patchType types.PatchType, data []byte, subresources ...string) (*v1.ResourceQuota, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ResourceQuota), err
}

func (s *resourceQuotaClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *resourceQuotaClient) AddHandler(ctx context.Context, name string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *resourceQuotaClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *resourceQuotaClient) AddLifecycle(ctx context.Context, name string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *resourceQuotaClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *resourceQuotaClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ResourceQuotaHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *resourceQuotaClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ResourceQuotaLifecycle) {
	sync := NewResourceQuotaLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ResourceQuotaIndexer func(obj *v1.ResourceQuota) ([]string, error)

type ResourceQuotaClientCache interface {
	Get(namespace, name string) (*v1.ResourceQuota, error)
	List(namespace string, selector labels.Selector) ([]*v1.ResourceQuota, error)

	Index(name string, indexer ResourceQuotaIndexer)
	GetIndexed(name, key string) ([]*v1.ResourceQuota, error)
}

type ResourceQuotaClient interface {
	Create(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error)
	Update(*v1.ResourceQuota) (*v1.ResourceQuota, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ResourceQuotaList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ResourceQuotaClientCache

	OnCreate(ctx context.Context, name string, sync ResourceQuotaChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ResourceQuotaChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ResourceQuotaChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ResourceQuotaInterface
}

type resourceQuotaClientCache struct {
	client *resourceQuotaClient2
}

type resourceQuotaClient2 struct {
	iface      ResourceQuotaInterface
	controller ResourceQuotaController
}

func (n *resourceQuotaClient2) Interface() ResourceQuotaInterface {
	return n.iface
}

func (n *resourceQuotaClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *resourceQuotaClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *resourceQuotaClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *resourceQuotaClient2) Create(obj *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	return n.iface.Create(obj)
}

func (n *resourceQuotaClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.ResourceQuota, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *resourceQuotaClient2) Update(obj *v1.ResourceQuota) (*v1.ResourceQuota, error) {
	return n.iface.Update(obj)
}

func (n *resourceQuotaClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *resourceQuotaClient2) List(namespace string, opts metav1.ListOptions) (*ResourceQuotaList, error) {
	return n.iface.List(opts)
}

func (n *resourceQuotaClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *resourceQuotaClientCache) Get(namespace, name string) (*v1.ResourceQuota, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *resourceQuotaClientCache) List(namespace string, selector labels.Selector) ([]*v1.ResourceQuota, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *resourceQuotaClient2) Cache() ResourceQuotaClientCache {
	n.loadController()
	return &resourceQuotaClientCache{
		client: n,
	}
}

func (n *resourceQuotaClient2) OnCreate(ctx context.Context, name string, sync ResourceQuotaChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &resourceQuotaLifecycleDelegate{create: sync})
}

func (n *resourceQuotaClient2) OnChange(ctx context.Context, name string, sync ResourceQuotaChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &resourceQuotaLifecycleDelegate{update: sync})
}

func (n *resourceQuotaClient2) OnRemove(ctx context.Context, name string, sync ResourceQuotaChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &resourceQuotaLifecycleDelegate{remove: sync})
}

func (n *resourceQuotaClientCache) Index(name string, indexer ResourceQuotaIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.ResourceQuota); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *resourceQuotaClientCache) GetIndexed(name, key string) ([]*v1.ResourceQuota, error) {
	var result []*v1.ResourceQuota
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.ResourceQuota); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *resourceQuotaClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type resourceQuotaLifecycleDelegate struct {
	create ResourceQuotaChangeHandlerFunc
	update ResourceQuotaChangeHandlerFunc
	remove ResourceQuotaChangeHandlerFunc
}

func (n *resourceQuotaLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *resourceQuotaLifecycleDelegate) Create(obj *v1.ResourceQuota) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *resourceQuotaLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *resourceQuotaLifecycleDelegate) Remove(obj *v1.ResourceQuota) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *resourceQuotaLifecycleDelegate) Updated(obj *v1.ResourceQuota) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
