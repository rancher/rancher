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
	ClusterUserAttributeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "ClusterUserAttribute",
	}
	ClusterUserAttributeResource = metav1.APIResource{
		Name:         "clusteruserattributes",
		SingularName: "clusteruserattribute",
		Namespaced:   true,

		Kind: ClusterUserAttributeGroupVersionKind.Kind,
	}

	ClusterUserAttributeGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "clusteruserattributes",
	}
)

func init() {
	resource.Put(ClusterUserAttributeGroupVersionResource)
}

func NewClusterUserAttribute(namespace, name string, obj ClusterUserAttribute) *ClusterUserAttribute {
	obj.APIVersion, obj.Kind = ClusterUserAttributeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type ClusterUserAttributeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterUserAttribute `json:"items"`
}

type ClusterUserAttributeHandlerFunc func(key string, obj *ClusterUserAttribute) (runtime.Object, error)

type ClusterUserAttributeChangeHandlerFunc func(obj *ClusterUserAttribute) (runtime.Object, error)

type ClusterUserAttributeLister interface {
	List(namespace string, selector labels.Selector) (ret []*ClusterUserAttribute, err error)
	Get(namespace, name string) (*ClusterUserAttribute, error)
}

type ClusterUserAttributeController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() ClusterUserAttributeLister
	AddHandler(ctx context.Context, name string, handler ClusterUserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterUserAttributeHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler ClusterUserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler ClusterUserAttributeHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type ClusterUserAttributeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*ClusterUserAttribute) (*ClusterUserAttribute, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterUserAttribute, error)
	Get(name string, opts metav1.GetOptions) (*ClusterUserAttribute, error)
	Update(*ClusterUserAttribute) (*ClusterUserAttribute, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*ClusterUserAttributeList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterUserAttributeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() ClusterUserAttributeController
	AddHandler(ctx context.Context, name string, sync ClusterUserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterUserAttributeHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle ClusterUserAttributeLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterUserAttributeLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterUserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterUserAttributeHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterUserAttributeLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterUserAttributeLifecycle)
}

type clusterUserAttributeLister struct {
	controller *clusterUserAttributeController
}

func (l *clusterUserAttributeLister) List(namespace string, selector labels.Selector) (ret []*ClusterUserAttribute, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*ClusterUserAttribute))
	})
	return
}

func (l *clusterUserAttributeLister) Get(namespace, name string) (*ClusterUserAttribute, error) {
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
			Group:    ClusterUserAttributeGroupVersionKind.Group,
			Resource: "clusterUserAttribute",
		}, key)
	}
	return obj.(*ClusterUserAttribute), nil
}

type clusterUserAttributeController struct {
	controller.GenericController
}

func (c *clusterUserAttributeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *clusterUserAttributeController) Lister() ClusterUserAttributeLister {
	return &clusterUserAttributeLister{
		controller: c,
	}
}

func (c *clusterUserAttributeController) AddHandler(ctx context.Context, name string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterUserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterUserAttributeController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterUserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterUserAttributeController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterUserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *clusterUserAttributeController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler ClusterUserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*ClusterUserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type clusterUserAttributeFactory struct {
}

func (c clusterUserAttributeFactory) Object() runtime.Object {
	return &ClusterUserAttribute{}
}

func (c clusterUserAttributeFactory) List() runtime.Object {
	return &ClusterUserAttributeList{}
}

func (s *clusterUserAttributeClient) Controller() ClusterUserAttributeController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.clusterUserAttributeControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(ClusterUserAttributeGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &clusterUserAttributeController{
		GenericController: genericController,
	}

	s.client.clusterUserAttributeControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type clusterUserAttributeClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   ClusterUserAttributeController
}

func (s *clusterUserAttributeClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *clusterUserAttributeClient) Create(o *ClusterUserAttribute) (*ClusterUserAttribute, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) Get(name string, opts metav1.GetOptions) (*ClusterUserAttribute, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*ClusterUserAttribute, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) Update(o *ClusterUserAttribute) (*ClusterUserAttribute, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *clusterUserAttributeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *clusterUserAttributeClient) List(opts metav1.ListOptions) (*ClusterUserAttributeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*ClusterUserAttributeList), err
}

func (s *clusterUserAttributeClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*ClusterUserAttributeList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*ClusterUserAttributeList), err
}

func (s *clusterUserAttributeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *clusterUserAttributeClient) Patch(o *ClusterUserAttribute, patchType types.PatchType, data []byte, subresources ...string) (*ClusterUserAttribute, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*ClusterUserAttribute), err
}

func (s *clusterUserAttributeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *clusterUserAttributeClient) AddHandler(ctx context.Context, name string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterUserAttributeClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterUserAttributeClient) AddLifecycle(ctx context.Context, name string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *clusterUserAttributeClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync ClusterUserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *clusterUserAttributeClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle ClusterUserAttributeLifecycle) {
	sync := NewClusterUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type ClusterUserAttributeIndexer func(obj *ClusterUserAttribute) ([]string, error)

type ClusterUserAttributeClientCache interface {
	Get(namespace, name string) (*ClusterUserAttribute, error)
	List(namespace string, selector labels.Selector) ([]*ClusterUserAttribute, error)

	Index(name string, indexer ClusterUserAttributeIndexer)
	GetIndexed(name, key string) ([]*ClusterUserAttribute, error)
}

type ClusterUserAttributeClient interface {
	Create(*ClusterUserAttribute) (*ClusterUserAttribute, error)
	Get(namespace, name string, opts metav1.GetOptions) (*ClusterUserAttribute, error)
	Update(*ClusterUserAttribute) (*ClusterUserAttribute, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*ClusterUserAttributeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() ClusterUserAttributeClientCache

	OnCreate(ctx context.Context, name string, sync ClusterUserAttributeChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync ClusterUserAttributeChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync ClusterUserAttributeChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() ClusterUserAttributeInterface
}

type clusterUserAttributeClientCache struct {
	client *clusterUserAttributeClient2
}

type clusterUserAttributeClient2 struct {
	iface      ClusterUserAttributeInterface
	controller ClusterUserAttributeController
}

func (n *clusterUserAttributeClient2) Interface() ClusterUserAttributeInterface {
	return n.iface
}

func (n *clusterUserAttributeClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *clusterUserAttributeClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *clusterUserAttributeClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *clusterUserAttributeClient2) Create(obj *ClusterUserAttribute) (*ClusterUserAttribute, error) {
	return n.iface.Create(obj)
}

func (n *clusterUserAttributeClient2) Get(namespace, name string, opts metav1.GetOptions) (*ClusterUserAttribute, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *clusterUserAttributeClient2) Update(obj *ClusterUserAttribute) (*ClusterUserAttribute, error) {
	return n.iface.Update(obj)
}

func (n *clusterUserAttributeClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *clusterUserAttributeClient2) List(namespace string, opts metav1.ListOptions) (*ClusterUserAttributeList, error) {
	return n.iface.List(opts)
}

func (n *clusterUserAttributeClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *clusterUserAttributeClientCache) Get(namespace, name string) (*ClusterUserAttribute, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *clusterUserAttributeClientCache) List(namespace string, selector labels.Selector) ([]*ClusterUserAttribute, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *clusterUserAttributeClient2) Cache() ClusterUserAttributeClientCache {
	n.loadController()
	return &clusterUserAttributeClientCache{
		client: n,
	}
}

func (n *clusterUserAttributeClient2) OnCreate(ctx context.Context, name string, sync ClusterUserAttributeChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &clusterUserAttributeLifecycleDelegate{create: sync})
}

func (n *clusterUserAttributeClient2) OnChange(ctx context.Context, name string, sync ClusterUserAttributeChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &clusterUserAttributeLifecycleDelegate{update: sync})
}

func (n *clusterUserAttributeClient2) OnRemove(ctx context.Context, name string, sync ClusterUserAttributeChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &clusterUserAttributeLifecycleDelegate{remove: sync})
}

func (n *clusterUserAttributeClientCache) Index(name string, indexer ClusterUserAttributeIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*ClusterUserAttribute); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *clusterUserAttributeClientCache) GetIndexed(name, key string) ([]*ClusterUserAttribute, error) {
	var result []*ClusterUserAttribute
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*ClusterUserAttribute); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *clusterUserAttributeClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type clusterUserAttributeLifecycleDelegate struct {
	create ClusterUserAttributeChangeHandlerFunc
	update ClusterUserAttributeChangeHandlerFunc
	remove ClusterUserAttributeChangeHandlerFunc
}

func (n *clusterUserAttributeLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *clusterUserAttributeLifecycleDelegate) Create(obj *ClusterUserAttribute) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *clusterUserAttributeLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *clusterUserAttributeLifecycleDelegate) Remove(obj *ClusterUserAttribute) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *clusterUserAttributeLifecycleDelegate) Updated(obj *ClusterUserAttribute) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
