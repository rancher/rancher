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
	AlertmanagerGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Alertmanager",
	}
	AlertmanagerResource = metav1.APIResource{
		Name:         "alertmanagers",
		SingularName: "alertmanager",
		Namespaced:   true,

		Kind: AlertmanagerGroupVersionKind.Kind,
	}

	AlertmanagerGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "alertmanagers",
	}
)

func init() {
	resource.Put(AlertmanagerGroupVersionResource)
}

func NewAlertmanager(namespace, name string, obj v1.Alertmanager) *v1.Alertmanager {
	obj.APIVersion, obj.Kind = AlertmanagerGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type AlertmanagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.Alertmanager `json:"items"`
}

type AlertmanagerHandlerFunc func(key string, obj *v1.Alertmanager) (runtime.Object, error)

type AlertmanagerChangeHandlerFunc func(obj *v1.Alertmanager) (runtime.Object, error)

type AlertmanagerLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.Alertmanager, err error)
	Get(namespace, name string) (*v1.Alertmanager, error)
}

type AlertmanagerController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() AlertmanagerLister
	AddHandler(ctx context.Context, name string, handler AlertmanagerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AlertmanagerHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler AlertmanagerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler AlertmanagerHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type AlertmanagerInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.Alertmanager) (*v1.Alertmanager, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Alertmanager, error)
	Get(name string, opts metav1.GetOptions) (*v1.Alertmanager, error)
	Update(*v1.Alertmanager) (*v1.Alertmanager, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*AlertmanagerList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*AlertmanagerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() AlertmanagerController
	AddHandler(ctx context.Context, name string, sync AlertmanagerHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AlertmanagerHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle AlertmanagerLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AlertmanagerLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AlertmanagerHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AlertmanagerHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AlertmanagerLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AlertmanagerLifecycle)
}

type alertmanagerLister struct {
	controller *alertmanagerController
}

func (l *alertmanagerLister) List(namespace string, selector labels.Selector) (ret []*v1.Alertmanager, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.Alertmanager))
	})
	return
}

func (l *alertmanagerLister) Get(namespace, name string) (*v1.Alertmanager, error) {
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
			Group:    AlertmanagerGroupVersionKind.Group,
			Resource: "alertmanager",
		}, key)
	}
	return obj.(*v1.Alertmanager), nil
}

type alertmanagerController struct {
	controller.GenericController
}

func (c *alertmanagerController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *alertmanagerController) Lister() AlertmanagerLister {
	return &alertmanagerLister{
		controller: c,
	}
}

func (c *alertmanagerController) AddHandler(ctx context.Context, name string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *alertmanagerController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *alertmanagerController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *alertmanagerController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler AlertmanagerHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.Alertmanager); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type alertmanagerFactory struct {
}

func (c alertmanagerFactory) Object() runtime.Object {
	return &v1.Alertmanager{}
}

func (c alertmanagerFactory) List() runtime.Object {
	return &AlertmanagerList{}
}

func (s *alertmanagerClient) Controller() AlertmanagerController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.alertmanagerControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(AlertmanagerGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &alertmanagerController{
		GenericController: genericController,
	}

	s.client.alertmanagerControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type alertmanagerClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   AlertmanagerController
}

func (s *alertmanagerClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *alertmanagerClient) Create(o *v1.Alertmanager) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) Get(name string, opts metav1.GetOptions) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) Update(o *v1.Alertmanager) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *alertmanagerClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *alertmanagerClient) List(opts metav1.ListOptions) (*AlertmanagerList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*AlertmanagerList), err
}

func (s *alertmanagerClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*AlertmanagerList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*AlertmanagerList), err
}

func (s *alertmanagerClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *alertmanagerClient) Patch(o *v1.Alertmanager, patchType types.PatchType, data []byte, subresources ...string) (*v1.Alertmanager, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.Alertmanager), err
}

func (s *alertmanagerClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *alertmanagerClient) AddHandler(ctx context.Context, name string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *alertmanagerClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *alertmanagerClient) AddLifecycle(ctx context.Context, name string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *alertmanagerClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *alertmanagerClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *alertmanagerClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync AlertmanagerHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *alertmanagerClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *alertmanagerClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle AlertmanagerLifecycle) {
	sync := NewAlertmanagerLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type AlertmanagerIndexer func(obj *v1.Alertmanager) ([]string, error)

type AlertmanagerClientCache interface {
	Get(namespace, name string) (*v1.Alertmanager, error)
	List(namespace string, selector labels.Selector) ([]*v1.Alertmanager, error)

	Index(name string, indexer AlertmanagerIndexer)
	GetIndexed(name, key string) ([]*v1.Alertmanager, error)
}

type AlertmanagerClient interface {
	Create(*v1.Alertmanager) (*v1.Alertmanager, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.Alertmanager, error)
	Update(*v1.Alertmanager) (*v1.Alertmanager, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*AlertmanagerList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() AlertmanagerClientCache

	OnCreate(ctx context.Context, name string, sync AlertmanagerChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync AlertmanagerChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync AlertmanagerChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() AlertmanagerInterface
}

type alertmanagerClientCache struct {
	client *alertmanagerClient2
}

type alertmanagerClient2 struct {
	iface      AlertmanagerInterface
	controller AlertmanagerController
}

func (n *alertmanagerClient2) Interface() AlertmanagerInterface {
	return n.iface
}

func (n *alertmanagerClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *alertmanagerClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *alertmanagerClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *alertmanagerClient2) Create(obj *v1.Alertmanager) (*v1.Alertmanager, error) {
	return n.iface.Create(obj)
}

func (n *alertmanagerClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.Alertmanager, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *alertmanagerClient2) Update(obj *v1.Alertmanager) (*v1.Alertmanager, error) {
	return n.iface.Update(obj)
}

func (n *alertmanagerClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *alertmanagerClient2) List(namespace string, opts metav1.ListOptions) (*AlertmanagerList, error) {
	return n.iface.List(opts)
}

func (n *alertmanagerClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *alertmanagerClientCache) Get(namespace, name string) (*v1.Alertmanager, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *alertmanagerClientCache) List(namespace string, selector labels.Selector) ([]*v1.Alertmanager, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *alertmanagerClient2) Cache() AlertmanagerClientCache {
	n.loadController()
	return &alertmanagerClientCache{
		client: n,
	}
}

func (n *alertmanagerClient2) OnCreate(ctx context.Context, name string, sync AlertmanagerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &alertmanagerLifecycleDelegate{create: sync})
}

func (n *alertmanagerClient2) OnChange(ctx context.Context, name string, sync AlertmanagerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &alertmanagerLifecycleDelegate{update: sync})
}

func (n *alertmanagerClient2) OnRemove(ctx context.Context, name string, sync AlertmanagerChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &alertmanagerLifecycleDelegate{remove: sync})
}

func (n *alertmanagerClientCache) Index(name string, indexer AlertmanagerIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.Alertmanager); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *alertmanagerClientCache) GetIndexed(name, key string) ([]*v1.Alertmanager, error) {
	var result []*v1.Alertmanager
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.Alertmanager); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *alertmanagerClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type alertmanagerLifecycleDelegate struct {
	create AlertmanagerChangeHandlerFunc
	update AlertmanagerChangeHandlerFunc
	remove AlertmanagerChangeHandlerFunc
}

func (n *alertmanagerLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *alertmanagerLifecycleDelegate) Create(obj *v1.Alertmanager) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *alertmanagerLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *alertmanagerLifecycleDelegate) Remove(obj *v1.Alertmanager) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *alertmanagerLifecycleDelegate) Updated(obj *v1.Alertmanager) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
