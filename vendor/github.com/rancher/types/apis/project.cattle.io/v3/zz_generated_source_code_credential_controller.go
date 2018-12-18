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
	SourceCodeCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SourceCodeCredential",
	}
	SourceCodeCredentialResource = metav1.APIResource{
		Name:         "sourcecodecredentials",
		SingularName: "sourcecodecredential",
		Namespaced:   true,

		Kind: SourceCodeCredentialGroupVersionKind.Kind,
	}
)

func NewSourceCodeCredential(namespace, name string, obj SourceCodeCredential) *SourceCodeCredential {
	obj.APIVersion, obj.Kind = SourceCodeCredentialGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type SourceCodeCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceCodeCredential
}

type SourceCodeCredentialHandlerFunc func(key string, obj *SourceCodeCredential) (runtime.Object, error)

type SourceCodeCredentialChangeHandlerFunc func(obj *SourceCodeCredential) (runtime.Object, error)

type SourceCodeCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*SourceCodeCredential, err error)
	Get(namespace, name string) (*SourceCodeCredential, error)
}

type SourceCodeCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() SourceCodeCredentialLister
	AddHandler(ctx context.Context, name string, handler SourceCodeCredentialHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler SourceCodeCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SourceCodeCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SourceCodeCredential) (*SourceCodeCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeCredential, error)
	Get(name string, opts metav1.GetOptions) (*SourceCodeCredential, error)
	Update(*SourceCodeCredential) (*SourceCodeCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SourceCodeCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SourceCodeCredentialController
	AddHandler(ctx context.Context, name string, sync SourceCodeCredentialHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeCredentialLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeCredentialHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeCredentialLifecycle)
}

type sourceCodeCredentialLister struct {
	controller *sourceCodeCredentialController
}

func (l *sourceCodeCredentialLister) List(namespace string, selector labels.Selector) (ret []*SourceCodeCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SourceCodeCredential))
	})
	return
}

func (l *sourceCodeCredentialLister) Get(namespace, name string) (*SourceCodeCredential, error) {
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
			Group:    SourceCodeCredentialGroupVersionKind.Group,
			Resource: "sourceCodeCredential",
		}, key)
	}
	return obj.(*SourceCodeCredential), nil
}

type sourceCodeCredentialController struct {
	controller.GenericController
}

func (c *sourceCodeCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *sourceCodeCredentialController) Lister() SourceCodeCredentialLister {
	return &sourceCodeCredentialLister{
		controller: c,
	}
}

func (c *sourceCodeCredentialController) AddHandler(ctx context.Context, name string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *sourceCodeCredentialController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler SourceCodeCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*SourceCodeCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type sourceCodeCredentialFactory struct {
}

func (c sourceCodeCredentialFactory) Object() runtime.Object {
	return &SourceCodeCredential{}
}

func (c sourceCodeCredentialFactory) List() runtime.Object {
	return &SourceCodeCredentialList{}
}

func (s *sourceCodeCredentialClient) Controller() SourceCodeCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.sourceCodeCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SourceCodeCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &sourceCodeCredentialController{
		GenericController: genericController,
	}

	s.client.sourceCodeCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type sourceCodeCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SourceCodeCredentialController
}

func (s *sourceCodeCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *sourceCodeCredentialClient) Create(o *SourceCodeCredential) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Get(name string, opts metav1.GetOptions) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Update(o *SourceCodeCredential) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *sourceCodeCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *sourceCodeCredentialClient) List(opts metav1.ListOptions) (*SourceCodeCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SourceCodeCredentialList), err
}

func (s *sourceCodeCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *sourceCodeCredentialClient) Patch(o *SourceCodeCredential, patchType types.PatchType, data []byte, subresources ...string) (*SourceCodeCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*SourceCodeCredential), err
}

func (s *sourceCodeCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *sourceCodeCredentialClient) AddHandler(ctx context.Context, name string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeCredentialClient) AddLifecycle(ctx context.Context, name string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync SourceCodeCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *sourceCodeCredentialClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle SourceCodeCredentialLifecycle) {
	sync := NewSourceCodeCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

type SourceCodeCredentialIndexer func(obj *SourceCodeCredential) ([]string, error)

type SourceCodeCredentialClientCache interface {
	Get(namespace, name string) (*SourceCodeCredential, error)
	List(namespace string, selector labels.Selector) ([]*SourceCodeCredential, error)

	Index(name string, indexer SourceCodeCredentialIndexer)
	GetIndexed(name, key string) ([]*SourceCodeCredential, error)
}

type SourceCodeCredentialClient interface {
	Create(*SourceCodeCredential) (*SourceCodeCredential, error)
	Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeCredential, error)
	Update(*SourceCodeCredential) (*SourceCodeCredential, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*SourceCodeCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() SourceCodeCredentialClientCache

	OnCreate(ctx context.Context, name string, sync SourceCodeCredentialChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync SourceCodeCredentialChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync SourceCodeCredentialChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() SourceCodeCredentialInterface
}

type sourceCodeCredentialClientCache struct {
	client *sourceCodeCredentialClient2
}

type sourceCodeCredentialClient2 struct {
	iface      SourceCodeCredentialInterface
	controller SourceCodeCredentialController
}

func (n *sourceCodeCredentialClient2) Interface() SourceCodeCredentialInterface {
	return n.iface
}

func (n *sourceCodeCredentialClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *sourceCodeCredentialClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *sourceCodeCredentialClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *sourceCodeCredentialClient2) Create(obj *SourceCodeCredential) (*SourceCodeCredential, error) {
	return n.iface.Create(obj)
}

func (n *sourceCodeCredentialClient2) Get(namespace, name string, opts metav1.GetOptions) (*SourceCodeCredential, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *sourceCodeCredentialClient2) Update(obj *SourceCodeCredential) (*SourceCodeCredential, error) {
	return n.iface.Update(obj)
}

func (n *sourceCodeCredentialClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *sourceCodeCredentialClient2) List(namespace string, opts metav1.ListOptions) (*SourceCodeCredentialList, error) {
	return n.iface.List(opts)
}

func (n *sourceCodeCredentialClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *sourceCodeCredentialClientCache) Get(namespace, name string) (*SourceCodeCredential, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *sourceCodeCredentialClientCache) List(namespace string, selector labels.Selector) ([]*SourceCodeCredential, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *sourceCodeCredentialClient2) Cache() SourceCodeCredentialClientCache {
	n.loadController()
	return &sourceCodeCredentialClientCache{
		client: n,
	}
}

func (n *sourceCodeCredentialClient2) OnCreate(ctx context.Context, name string, sync SourceCodeCredentialChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &sourceCodeCredentialLifecycleDelegate{create: sync})
}

func (n *sourceCodeCredentialClient2) OnChange(ctx context.Context, name string, sync SourceCodeCredentialChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &sourceCodeCredentialLifecycleDelegate{update: sync})
}

func (n *sourceCodeCredentialClient2) OnRemove(ctx context.Context, name string, sync SourceCodeCredentialChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &sourceCodeCredentialLifecycleDelegate{remove: sync})
}

func (n *sourceCodeCredentialClientCache) Index(name string, indexer SourceCodeCredentialIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*SourceCodeCredential); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *sourceCodeCredentialClientCache) GetIndexed(name, key string) ([]*SourceCodeCredential, error) {
	var result []*SourceCodeCredential
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*SourceCodeCredential); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *sourceCodeCredentialClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type sourceCodeCredentialLifecycleDelegate struct {
	create SourceCodeCredentialChangeHandlerFunc
	update SourceCodeCredentialChangeHandlerFunc
	remove SourceCodeCredentialChangeHandlerFunc
}

func (n *sourceCodeCredentialLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *sourceCodeCredentialLifecycleDelegate) Create(obj *SourceCodeCredential) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *sourceCodeCredentialLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *sourceCodeCredentialLifecycleDelegate) Remove(obj *SourceCodeCredential) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *sourceCodeCredentialLifecycleDelegate) Updated(obj *SourceCodeCredential) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
