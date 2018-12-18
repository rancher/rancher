package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
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
	SSHAuthGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SSHAuth",
	}
	SSHAuthResource = metav1.APIResource{
		Name:         "sshauths",
		SingularName: "sshauth",
		Namespaced:   true,

		Kind: SSHAuthGroupVersionKind.Kind,
	}
)

func NewSSHAuth(namespace, name string, obj SSHAuth) *SSHAuth {
	obj.APIVersion, obj.Kind = SSHAuthGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SSHAuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHAuth
}

type SSHAuthHandlerFunc func(key string, obj *SSHAuth) (runtime.Object, error)

type SSHAuthChangeHandlerFunc func(obj *SSHAuth) (runtime.Object, error)

type SSHAuthLister interface {
	List(namespace string, selector labels.Selector) (ret []*SSHAuth, err error)
	Get(namespace, name string) (*SSHAuth, error)
}

type SSHAuthController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SSHAuthLister
	AddHandler(ctx context.Context, name string, handler SSHAuthHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SSHAuthHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SSHAuthInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SSHAuth) (*SSHAuth, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SSHAuth, error)
	Get(name string, opts metav1.GetOptions) (*SSHAuth, error)
	Update(*SSHAuth) (*SSHAuth, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SSHAuthController
	AddHandler(ctx context.Context, name string, sync SSHAuthHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SSHAuthLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SSHAuthHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SSHAuthLifecycle)
}

type sshAuthLister struct {
	controller *sshAuthController
}

func (l *sshAuthLister) List(namespace string, selector labels.Selector) (ret []*SSHAuth, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SSHAuth))
	})
	return
}

func (l *sshAuthLister) Get(namespace, name string) (*SSHAuth, error) {
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
			Group:    SSHAuthGroupVersionKind.Group,
			Resource: "sshAuth",
		}, key)
	}
	return obj.(*SSHAuth), nil
}

type sshAuthController struct {
	controller.GenericController
}

func (c *sshAuthController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sshAuthController) Lister() SSHAuthLister {
	return &sshAuthLister{
		controller: c,
	}
}

func (c *sshAuthController) AddHandler(ctx context.Context, name string, handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SSHAuth); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sshAuthController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SSHAuthHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SSHAuth); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sshAuthFactory struct {
}

func (c sshAuthFactory) Object() runtime.Object {
	return &SSHAuth{}
}

func (c sshAuthFactory) List() runtime.Object {
	return &SSHAuthList{}
}

func (s *sshAuthClient) Controller() SSHAuthController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sshAuthControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SSHAuthGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sshAuthController{
		GenericController: genericController,
	}

	s.client.sshAuthControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sshAuthClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SSHAuthController
}

func (s *sshAuthClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sshAuthClient) Create(o *SSHAuth) (*SSHAuth, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Get(name string, opts metav1.GetOptions) (*SSHAuth, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SSHAuth, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Update(o *SSHAuth) (*SSHAuth, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sshAuthClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sshAuthClient) List(opts metav1.ListOptions) (*SSHAuthList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SSHAuthList), err
}

func (s *sshAuthClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sshAuthClient) Patch(o *SSHAuth, patchType types.PatchType, data []byte, subresources ...string) (*SSHAuth, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SSHAuth), err
}

func (s *sshAuthClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sshAuthClient) AddHandler(ctx context.Context, name string, sync SSHAuthHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sshAuthClient) AddLifecycle(ctx context.Context, name string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sshAuthClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SSHAuthHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sshAuthClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SSHAuthLifecycle) {
	sync := NewSSHAuthLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type SSHAuthIndexer func(obj *SSHAuth) ([]string, error)

type SSHAuthClientCache interface {
	Get(namespace, name string) (*SSHAuth, error)
	List(namespace string, selector labels.Selector) ([]*SSHAuth, error)

	Index(name string, indexer SSHAuthIndexer)
	GetIndexed(name, key string) ([]*SSHAuth, error)
}

type SSHAuthClient interface {
	Create(*SSHAuth) (*SSHAuth, error)
	Get(namespace, name string, opts metav1.GetOptions) (*SSHAuth, error)
	Update(*SSHAuth) (*SSHAuth, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*SSHAuthList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() SSHAuthClientCache

	OnCreate(ctx context.Context, name string, sync SSHAuthChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync SSHAuthChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync SSHAuthChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() SSHAuthInterface
}

type sshAuthClientCache struct {
	client *sshAuthClient2
}

type sshAuthClient2 struct {
	iface      SSHAuthInterface
	controller SSHAuthController
}

func (n *sshAuthClient2) Interface() SSHAuthInterface {
	return n.iface
}

func (n *sshAuthClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *sshAuthClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *sshAuthClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *sshAuthClient2) Create(obj *SSHAuth) (*SSHAuth, error) {
	return n.iface.Create(obj)
}

func (n *sshAuthClient2) Get(namespace, name string, opts metav1.GetOptions) (*SSHAuth, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *sshAuthClient2) Update(obj *SSHAuth) (*SSHAuth, error) {
	return n.iface.Update(obj)
}

func (n *sshAuthClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *sshAuthClient2) List(namespace string, opts metav1.ListOptions) (*SSHAuthList, error) {
	return n.iface.List(opts)
}

func (n *sshAuthClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *sshAuthClientCache) Get(namespace, name string) (*SSHAuth, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *sshAuthClientCache) List(namespace string, selector labels.Selector) ([]*SSHAuth, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *sshAuthClient2) Cache() SSHAuthClientCache {
	n.loadController()
	return &sshAuthClientCache{
		client: n,
	}
}

func (n *sshAuthClient2) OnCreate(ctx context.Context, name string, sync SSHAuthChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &sshAuthLifecycleDelegate{create: sync})
}

func (n *sshAuthClient2) OnChange(ctx context.Context, name string, sync SSHAuthChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &sshAuthLifecycleDelegate{update: sync})
}

func (n *sshAuthClient2) OnRemove(ctx context.Context, name string, sync SSHAuthChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &sshAuthLifecycleDelegate{remove: sync})
}

func (n *sshAuthClientCache) Index(name string, indexer SSHAuthIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*SSHAuth); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *sshAuthClientCache) GetIndexed(name, key string) ([]*SSHAuth, error) {
	var result []*SSHAuth
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*SSHAuth); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *sshAuthClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type sshAuthLifecycleDelegate struct {
	create SSHAuthChangeHandlerFunc
	update SSHAuthChangeHandlerFunc
	remove SSHAuthChangeHandlerFunc
}

func (n *sshAuthLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *sshAuthLifecycleDelegate) Create(obj *SSHAuth) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *sshAuthLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *sshAuthLifecycleDelegate) Remove(obj *SSHAuth) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *sshAuthLifecycleDelegate) Updated(obj *SSHAuth) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
