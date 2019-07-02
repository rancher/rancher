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
	UserAttributeGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "UserAttribute",
	}
	UserAttributeResource = metav1.APIResource{
		Name:         "userattributes",
		SingularName: "userattribute",
		Namespaced:   false,
		Kind:         UserAttributeGroupVersionKind.Kind,
	}

	UserAttributeGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "userattributes",
	}
)

func init() {
	resource.Put(UserAttributeGroupVersionResource)
}

func NewUserAttribute(namespace, name string, obj UserAttribute) *UserAttribute {
	obj.APIVersion, obj.Kind = UserAttributeGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type UserAttributeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserAttribute `json:"items"`
}

type UserAttributeHandlerFunc func(key string, obj *UserAttribute) (runtime.Object, error)

type UserAttributeChangeHandlerFunc func(obj *UserAttribute) (runtime.Object, error)

type UserAttributeLister interface {
	List(namespace string, selector labels.Selector) (ret []*UserAttribute, err error)
	Get(namespace, name string) (*UserAttribute, error)
}

type UserAttributeController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() UserAttributeLister
	AddHandler(ctx context.Context, name string, handler UserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserAttributeHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler UserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler UserAttributeHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type UserAttributeInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*UserAttribute) (*UserAttribute, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*UserAttribute, error)
	Get(name string, opts metav1.GetOptions) (*UserAttribute, error)
	Update(*UserAttribute) (*UserAttribute, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*UserAttributeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() UserAttributeController
	AddHandler(ctx context.Context, name string, sync UserAttributeHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserAttributeHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle UserAttributeLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle UserAttributeLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserAttributeHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync UserAttributeHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserAttributeLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle UserAttributeLifecycle)
}

type userAttributeLister struct {
	controller *userAttributeController
}

func (l *userAttributeLister) List(namespace string, selector labels.Selector) (ret []*UserAttribute, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*UserAttribute))
	})
	return
}

func (l *userAttributeLister) Get(namespace, name string) (*UserAttribute, error) {
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
			Group:    UserAttributeGroupVersionKind.Group,
			Resource: "userAttribute",
		}, key)
	}
	return obj.(*UserAttribute), nil
}

type userAttributeController struct {
	controller.GenericController
}

func (c *userAttributeController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *userAttributeController) Lister() UserAttributeLister {
	return &userAttributeLister{
		controller: c,
	}
}

func (c *userAttributeController) AddHandler(ctx context.Context, name string, handler UserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userAttributeController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler UserAttributeHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userAttributeController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler UserAttributeHandlerFunc) {
	resource.PutClusterScoped(UserAttributeGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *userAttributeController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler UserAttributeHandlerFunc) {
	resource.PutClusterScoped(UserAttributeGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*UserAttribute); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type userAttributeFactory struct {
}

func (c userAttributeFactory) Object() runtime.Object {
	return &UserAttribute{}
}

func (c userAttributeFactory) List() runtime.Object {
	return &UserAttributeList{}
}

func (s *userAttributeClient) Controller() UserAttributeController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.userAttributeControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(UserAttributeGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &userAttributeController{
		GenericController: genericController,
	}

	s.client.userAttributeControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type userAttributeClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   UserAttributeController
}

func (s *userAttributeClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *userAttributeClient) Create(o *UserAttribute) (*UserAttribute, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) Get(name string, opts metav1.GetOptions) (*UserAttribute, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*UserAttribute, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) Update(o *UserAttribute) (*UserAttribute, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *userAttributeClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *userAttributeClient) List(opts metav1.ListOptions) (*UserAttributeList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*UserAttributeList), err
}

func (s *userAttributeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *userAttributeClient) Patch(o *UserAttribute, patchType types.PatchType, data []byte, subresources ...string) (*UserAttribute, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*UserAttribute), err
}

func (s *userAttributeClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *userAttributeClient) AddHandler(ctx context.Context, name string, sync UserAttributeHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userAttributeClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync UserAttributeHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *userAttributeClient) AddLifecycle(ctx context.Context, name string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *userAttributeClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *userAttributeClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync UserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userAttributeClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync UserAttributeHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *userAttributeClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *userAttributeClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle UserAttributeLifecycle) {
	sync := NewUserAttributeLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type UserAttributeIndexer func(obj *UserAttribute) ([]string, error)

type UserAttributeClientCache interface {
	Get(namespace, name string) (*UserAttribute, error)
	List(namespace string, selector labels.Selector) ([]*UserAttribute, error)

	Index(name string, indexer UserAttributeIndexer)
	GetIndexed(name, key string) ([]*UserAttribute, error)
}

type UserAttributeClient interface {
	Create(*UserAttribute) (*UserAttribute, error)
	Get(namespace, name string, opts metav1.GetOptions) (*UserAttribute, error)
	Update(*UserAttribute) (*UserAttribute, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*UserAttributeList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() UserAttributeClientCache

	OnCreate(ctx context.Context, name string, sync UserAttributeChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync UserAttributeChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync UserAttributeChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() UserAttributeInterface
}

type userAttributeClientCache struct {
	client *userAttributeClient2
}

type userAttributeClient2 struct {
	iface      UserAttributeInterface
	controller UserAttributeController
}

func (n *userAttributeClient2) Interface() UserAttributeInterface {
	return n.iface
}

func (n *userAttributeClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *userAttributeClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *userAttributeClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *userAttributeClient2) Create(obj *UserAttribute) (*UserAttribute, error) {
	return n.iface.Create(obj)
}

func (n *userAttributeClient2) Get(namespace, name string, opts metav1.GetOptions) (*UserAttribute, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *userAttributeClient2) Update(obj *UserAttribute) (*UserAttribute, error) {
	return n.iface.Update(obj)
}

func (n *userAttributeClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *userAttributeClient2) List(namespace string, opts metav1.ListOptions) (*UserAttributeList, error) {
	return n.iface.List(opts)
}

func (n *userAttributeClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *userAttributeClientCache) Get(namespace, name string) (*UserAttribute, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *userAttributeClientCache) List(namespace string, selector labels.Selector) ([]*UserAttribute, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *userAttributeClient2) Cache() UserAttributeClientCache {
	n.loadController()
	return &userAttributeClientCache{
		client: n,
	}
}

func (n *userAttributeClient2) OnCreate(ctx context.Context, name string, sync UserAttributeChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &userAttributeLifecycleDelegate{create: sync})
}

func (n *userAttributeClient2) OnChange(ctx context.Context, name string, sync UserAttributeChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &userAttributeLifecycleDelegate{update: sync})
}

func (n *userAttributeClient2) OnRemove(ctx context.Context, name string, sync UserAttributeChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &userAttributeLifecycleDelegate{remove: sync})
}

func (n *userAttributeClientCache) Index(name string, indexer UserAttributeIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*UserAttribute); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *userAttributeClientCache) GetIndexed(name, key string) ([]*UserAttribute, error) {
	var result []*UserAttribute
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*UserAttribute); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *userAttributeClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type userAttributeLifecycleDelegate struct {
	create UserAttributeChangeHandlerFunc
	update UserAttributeChangeHandlerFunc
	remove UserAttributeChangeHandlerFunc
}

func (n *userAttributeLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *userAttributeLifecycleDelegate) Create(obj *UserAttribute) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *userAttributeLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *userAttributeLifecycleDelegate) Remove(obj *UserAttribute) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *userAttributeLifecycleDelegate) Updated(obj *UserAttribute) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
