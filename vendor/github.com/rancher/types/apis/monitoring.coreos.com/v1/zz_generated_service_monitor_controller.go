package v1

import (
	"context"

	v1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
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
	ServiceMonitorGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ServiceMonitor",
	}
	ServiceMonitorResource = metav1.APIResource{
		Name:         "servicemonitors",
		SingularName: "servicemonitor",
		Namespaced:   true,

		Kind: ServiceMonitorGroupVersionKind.Kind,
	}

	ServiceMonitorGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "servicemonitors",
	}
)

func init() {
	resource.Put(ServiceMonitorGroupVersionResource)
}

func NewServiceMonitor(namespace, name string, obj v1.ServiceMonitor) *v1.ServiceMonitor {
	obj.APIVersion, obj.Kind = ServiceMonitorGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ServiceMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.ServiceMonitor `json:"items"`
}

type ServiceMonitorHandlerFunc func(key string, obj *v1.ServiceMonitor) (runtime.Object, error)

type ServiceMonitorChangeHandlerFunc func(obj *v1.ServiceMonitor) (runtime.Object, error)

type ServiceMonitorLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.ServiceMonitor, err error)
	Get(namespace, name string) (*v1.ServiceMonitor, error)
}

type ServiceMonitorController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ServiceMonitorLister
	AddHandler(ctx context.Context, name string, handler ServiceMonitorHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceMonitorHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ServiceMonitorHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ServiceMonitorHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ServiceMonitorInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.ServiceMonitor) (*v1.ServiceMonitor, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error)
	Get(name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error)
	Update(*v1.ServiceMonitor) (*v1.ServiceMonitor, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ServiceMonitorList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ServiceMonitorList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ServiceMonitorController
	AddHandler(ctx context.Context, name string, sync ServiceMonitorHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceMonitorHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ServiceMonitorLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceMonitorLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceMonitorHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceMonitorHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceMonitorLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceMonitorLifecycle)
}

type serviceMonitorLister struct {
	controller *serviceMonitorController
}

func (l *serviceMonitorLister) List(namespace string, selector labels.Selector) (ret []*v1.ServiceMonitor, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.ServiceMonitor))
	})
	return
}

func (l *serviceMonitorLister) Get(namespace, name string) (*v1.ServiceMonitor, error) {
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
			Group:    ServiceMonitorGroupVersionKind.Group,
			Resource: "serviceMonitor",
		}, key)
	}
	return obj.(*v1.ServiceMonitor), nil
}

type serviceMonitorController struct {
	controller.GenericController
}

func (c *serviceMonitorController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *serviceMonitorController) Lister() ServiceMonitorLister {
	return &serviceMonitorLister{
		controller: c,
	}
}

func (c *serviceMonitorController) AddHandler(ctx context.Context, name string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceMonitorController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceMonitorController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *serviceMonitorController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ServiceMonitorHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.ServiceMonitor); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type serviceMonitorFactory struct {
}

func (c serviceMonitorFactory) Object() runtime.Object {
	return &v1.ServiceMonitor{}
}

func (c serviceMonitorFactory) List() runtime.Object {
	return &ServiceMonitorList{}
}

func (s *serviceMonitorClient) Controller() ServiceMonitorController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.serviceMonitorControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ServiceMonitorGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &serviceMonitorController{
		GenericController: genericController,
	}

	s.client.serviceMonitorControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type serviceMonitorClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ServiceMonitorController
}

func (s *serviceMonitorClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *serviceMonitorClient) Create(o *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) Get(name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) Update(o *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *serviceMonitorClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *serviceMonitorClient) List(opts metav1.ListOptions) (*ServiceMonitorList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ServiceMonitorList), err
}

func (s *serviceMonitorClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ServiceMonitorList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ServiceMonitorList), err
}

func (s *serviceMonitorClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *serviceMonitorClient) Patch(o *v1.ServiceMonitor, patchType types.PatchType, data []byte, subresources ...string) (*v1.ServiceMonitor, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.ServiceMonitor), err
}

func (s *serviceMonitorClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *serviceMonitorClient) AddHandler(ctx context.Context, name string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceMonitorClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceMonitorClient) AddLifecycle(ctx context.Context, name string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *serviceMonitorClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *serviceMonitorClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceMonitorClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ServiceMonitorHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *serviceMonitorClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *serviceMonitorClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ServiceMonitorLifecycle) {
	sync := NewServiceMonitorLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ServiceMonitorIndexer func(obj *v1.ServiceMonitor) ([]string, error)

type ServiceMonitorClientCache interface {
	Get(namespace, name string) (*v1.ServiceMonitor, error)
	List(namespace string, selector labels.Selector) ([]*v1.ServiceMonitor, error)

	Index(name string, indexer ServiceMonitorIndexer)
	GetIndexed(name, key string) ([]*v1.ServiceMonitor, error)
}

type ServiceMonitorClient interface {
	Create(*v1.ServiceMonitor) (*v1.ServiceMonitor, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error)
	Update(*v1.ServiceMonitor) (*v1.ServiceMonitor, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ServiceMonitorList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ServiceMonitorClientCache

	OnCreate(ctx context.Context, name string, sync ServiceMonitorChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ServiceMonitorChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ServiceMonitorChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ServiceMonitorInterface
}

type serviceMonitorClientCache struct {
	client *serviceMonitorClient2
}

type serviceMonitorClient2 struct {
	iface      ServiceMonitorInterface
	controller ServiceMonitorController
}

func (n *serviceMonitorClient2) Interface() ServiceMonitorInterface {
	return n.iface
}

func (n *serviceMonitorClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *serviceMonitorClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *serviceMonitorClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *serviceMonitorClient2) Create(obj *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	return n.iface.Create(obj)
}

func (n *serviceMonitorClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.ServiceMonitor, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *serviceMonitorClient2) Update(obj *v1.ServiceMonitor) (*v1.ServiceMonitor, error) {
	return n.iface.Update(obj)
}

func (n *serviceMonitorClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *serviceMonitorClient2) List(namespace string, opts metav1.ListOptions) (*ServiceMonitorList, error) {
	return n.iface.List(opts)
}

func (n *serviceMonitorClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *serviceMonitorClientCache) Get(namespace, name string) (*v1.ServiceMonitor, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *serviceMonitorClientCache) List(namespace string, selector labels.Selector) ([]*v1.ServiceMonitor, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *serviceMonitorClient2) Cache() ServiceMonitorClientCache {
	n.loadController()
	return &serviceMonitorClientCache{
		client: n,
	}
}

func (n *serviceMonitorClient2) OnCreate(ctx context.Context, name string, sync ServiceMonitorChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &serviceMonitorLifecycleDelegate{create: sync})
}

func (n *serviceMonitorClient2) OnChange(ctx context.Context, name string, sync ServiceMonitorChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &serviceMonitorLifecycleDelegate{update: sync})
}

func (n *serviceMonitorClient2) OnRemove(ctx context.Context, name string, sync ServiceMonitorChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &serviceMonitorLifecycleDelegate{remove: sync})
}

func (n *serviceMonitorClientCache) Index(name string, indexer ServiceMonitorIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.ServiceMonitor); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *serviceMonitorClientCache) GetIndexed(name, key string) ([]*v1.ServiceMonitor, error) {
	var result []*v1.ServiceMonitor
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.ServiceMonitor); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *serviceMonitorClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type serviceMonitorLifecycleDelegate struct {
	create ServiceMonitorChangeHandlerFunc
	update ServiceMonitorChangeHandlerFunc
	remove ServiceMonitorChangeHandlerFunc
}

func (n *serviceMonitorLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *serviceMonitorLifecycleDelegate) Create(obj *v1.ServiceMonitor) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *serviceMonitorLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *serviceMonitorLifecycleDelegate) Remove(obj *v1.ServiceMonitor) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *serviceMonitorLifecycleDelegate) Updated(obj *v1.ServiceMonitor) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
