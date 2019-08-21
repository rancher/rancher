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
	EtcdBackupGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "EtcdBackup",
	}
	EtcdBackupResource = metav1.APIResource{
		Name:         "etcdbackups",
		SingularName: "etcdbackup",
		Namespaced:   true,

		Kind: EtcdBackupGroupVersionKind.Kind,
	}

	EtcdBackupGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "etcdbackups",
	}
)

func init() {
	resource.Put(EtcdBackupGroupVersionResource)
}

func NewEtcdBackup(namespace, name string, obj EtcdBackup) *EtcdBackup {
	obj.APIVersion, obj.Kind = EtcdBackupGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdBackup `json:"items"`
}

type EtcdBackupHandlerFunc func(key string, obj *EtcdBackup) (runtime.Object, error)

type EtcdBackupChangeHandlerFunc func(obj *EtcdBackup) (runtime.Object, error)

type EtcdBackupLister interface {
	List(namespace string, selector labels.Selector) (ret []*EtcdBackup, err error)
	Get(namespace, name string) (*EtcdBackup, error)
}

type EtcdBackupController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() EtcdBackupLister
	AddHandler(ctx context.Context, name string, handler EtcdBackupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EtcdBackupHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler EtcdBackupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler EtcdBackupHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type EtcdBackupInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*EtcdBackup) (*EtcdBackup, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*EtcdBackup, error)
	Get(name string, opts metav1.GetOptions) (*EtcdBackup, error)
	Update(*EtcdBackup) (*EtcdBackup, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*EtcdBackupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() EtcdBackupController
	AddHandler(ctx context.Context, name string, sync EtcdBackupHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EtcdBackupHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle EtcdBackupLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EtcdBackupLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EtcdBackupHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EtcdBackupHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EtcdBackupLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EtcdBackupLifecycle)
}

type etcdBackupLister struct {
	controller *etcdBackupController
}

func (l *etcdBackupLister) List(namespace string, selector labels.Selector) (ret []*EtcdBackup, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*EtcdBackup))
	})
	return
}

func (l *etcdBackupLister) Get(namespace, name string) (*EtcdBackup, error) {
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
			Group:    EtcdBackupGroupVersionKind.Group,
			Resource: "etcdBackup",
		}, key)
	}
	return obj.(*EtcdBackup), nil
}

type etcdBackupController struct {
	controller.GenericController
}

func (c *etcdBackupController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *etcdBackupController) Lister() EtcdBackupLister {
	return &etcdBackupLister{
		controller: c,
	}
}

func (c *etcdBackupController) AddHandler(ctx context.Context, name string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*EtcdBackup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *etcdBackupController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*EtcdBackup); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *etcdBackupController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*EtcdBackup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *etcdBackupController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler EtcdBackupHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*EtcdBackup); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type etcdBackupFactory struct {
}

func (c etcdBackupFactory) Object() runtime.Object {
	return &EtcdBackup{}
}

func (c etcdBackupFactory) List() runtime.Object {
	return &EtcdBackupList{}
}

func (s *etcdBackupClient) Controller() EtcdBackupController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.etcdBackupControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(EtcdBackupGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &etcdBackupController{
		GenericController: genericController,
	}

	s.client.etcdBackupControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type etcdBackupClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   EtcdBackupController
}

func (s *etcdBackupClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *etcdBackupClient) Create(o *EtcdBackup) (*EtcdBackup, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*EtcdBackup), err
}

func (s *etcdBackupClient) Get(name string, opts metav1.GetOptions) (*EtcdBackup, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*EtcdBackup), err
}

func (s *etcdBackupClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*EtcdBackup, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*EtcdBackup), err
}

func (s *etcdBackupClient) Update(o *EtcdBackup) (*EtcdBackup, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*EtcdBackup), err
}

func (s *etcdBackupClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *etcdBackupClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *etcdBackupClient) List(opts metav1.ListOptions) (*EtcdBackupList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*EtcdBackupList), err
}

func (s *etcdBackupClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *etcdBackupClient) Patch(o *EtcdBackup, patchType types.PatchType, data []byte, subresources ...string) (*EtcdBackup, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*EtcdBackup), err
}

func (s *etcdBackupClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *etcdBackupClient) AddHandler(ctx context.Context, name string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *etcdBackupClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *etcdBackupClient) AddLifecycle(ctx context.Context, name string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *etcdBackupClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *etcdBackupClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *etcdBackupClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync EtcdBackupHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *etcdBackupClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *etcdBackupClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle EtcdBackupLifecycle) {
	sync := NewEtcdBackupLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type EtcdBackupIndexer func(obj *EtcdBackup) ([]string, error)

type EtcdBackupClientCache interface {
	Get(namespace, name string) (*EtcdBackup, error)
	List(namespace string, selector labels.Selector) ([]*EtcdBackup, error)

	Index(name string, indexer EtcdBackupIndexer)
	GetIndexed(name, key string) ([]*EtcdBackup, error)
}

type EtcdBackupClient interface {
	Create(*EtcdBackup) (*EtcdBackup, error)
	Get(namespace, name string, opts metav1.GetOptions) (*EtcdBackup, error)
	Update(*EtcdBackup) (*EtcdBackup, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*EtcdBackupList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() EtcdBackupClientCache

	OnCreate(ctx context.Context, name string, sync EtcdBackupChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync EtcdBackupChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync EtcdBackupChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() EtcdBackupInterface
}

type etcdBackupClientCache struct {
	client *etcdBackupClient2
}

type etcdBackupClient2 struct {
	iface      EtcdBackupInterface
	controller EtcdBackupController
}

func (n *etcdBackupClient2) Interface() EtcdBackupInterface {
	return n.iface
}

func (n *etcdBackupClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *etcdBackupClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *etcdBackupClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *etcdBackupClient2) Create(obj *EtcdBackup) (*EtcdBackup, error) {
	return n.iface.Create(obj)
}

func (n *etcdBackupClient2) Get(namespace, name string, opts metav1.GetOptions) (*EtcdBackup, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *etcdBackupClient2) Update(obj *EtcdBackup) (*EtcdBackup, error) {
	return n.iface.Update(obj)
}

func (n *etcdBackupClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *etcdBackupClient2) List(namespace string, opts metav1.ListOptions) (*EtcdBackupList, error) {
	return n.iface.List(opts)
}

func (n *etcdBackupClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *etcdBackupClientCache) Get(namespace, name string) (*EtcdBackup, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *etcdBackupClientCache) List(namespace string, selector labels.Selector) ([]*EtcdBackup, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *etcdBackupClient2) Cache() EtcdBackupClientCache {
	n.loadController()
	return &etcdBackupClientCache{
		client: n,
	}
}

func (n *etcdBackupClient2) OnCreate(ctx context.Context, name string, sync EtcdBackupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &etcdBackupLifecycleDelegate{create: sync})
}

func (n *etcdBackupClient2) OnChange(ctx context.Context, name string, sync EtcdBackupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &etcdBackupLifecycleDelegate{update: sync})
}

func (n *etcdBackupClient2) OnRemove(ctx context.Context, name string, sync EtcdBackupChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &etcdBackupLifecycleDelegate{remove: sync})
}

func (n *etcdBackupClientCache) Index(name string, indexer EtcdBackupIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*EtcdBackup); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *etcdBackupClientCache) GetIndexed(name, key string) ([]*EtcdBackup, error) {
	var result []*EtcdBackup
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*EtcdBackup); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *etcdBackupClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type etcdBackupLifecycleDelegate struct {
	create EtcdBackupChangeHandlerFunc
	update EtcdBackupChangeHandlerFunc
	remove EtcdBackupChangeHandlerFunc
}

func (n *etcdBackupLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *etcdBackupLifecycleDelegate) Create(obj *EtcdBackup) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *etcdBackupLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *etcdBackupLifecycleDelegate) Remove(obj *EtcdBackup) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *etcdBackupLifecycleDelegate) Updated(obj *EtcdBackup) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
