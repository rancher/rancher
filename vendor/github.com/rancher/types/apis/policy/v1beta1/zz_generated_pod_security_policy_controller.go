package v1beta1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/policy/v1beta1"
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
	PodSecurityPolicyGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicy",
	}
	PodSecurityPolicyResource = metav1.APIResource{
		Name:         "podsecuritypolicies",
		SingularName: "podsecuritypolicy",
		Namespaced:   false,
		Kind:         PodSecurityPolicyGroupVersionKind.Kind,
	}

	PodSecurityPolicyGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "podsecuritypolicies",
	}
)

func init() {
	resource.Put(PodSecurityPolicyGroupVersionResource)
}

func NewPodSecurityPolicy(namespace, name string, obj v1beta1.PodSecurityPolicy) *v1beta1.PodSecurityPolicy {
	obj.APIVersion, obj.Kind = PodSecurityPolicyGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodSecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta1.PodSecurityPolicy `json:"items"`
}

type PodSecurityPolicyHandlerFunc func(key string, obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)

type PodSecurityPolicyChangeHandlerFunc func(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error)

type PodSecurityPolicyLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error)
	Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error)
}

type PodSecurityPolicyController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyLister
	AddHandler(ctx context.Context, name string, handler PodSecurityPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodSecurityPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodSecurityPolicyHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodSecurityPolicyInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Get(name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Update(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodSecurityPolicyList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyController
	AddHandler(ctx context.Context, name string, sync PodSecurityPolicyHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyLifecycle)
}

type podSecurityPolicyLister struct {
	controller *podSecurityPolicyController
}

func (l *podSecurityPolicyLister) List(namespace string, selector labels.Selector) (ret []*v1beta1.PodSecurityPolicy, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta1.PodSecurityPolicy))
	})
	return
}

func (l *podSecurityPolicyLister) Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error) {
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
			Group:    PodSecurityPolicyGroupVersionKind.Group,
			Resource: "podSecurityPolicy",
		}, key)
	}
	return obj.(*v1beta1.PodSecurityPolicy), nil
}

type podSecurityPolicyController struct {
	controller.GenericController
}

func (c *podSecurityPolicyController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podSecurityPolicyController) Lister() PodSecurityPolicyLister {
	return &podSecurityPolicyLister{
		controller: c,
	}
}

func (c *podSecurityPolicyController) AddHandler(ctx context.Context, name string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodSecurityPolicyHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podSecurityPolicyFactory struct {
}

func (c podSecurityPolicyFactory) Object() runtime.Object {
	return &v1beta1.PodSecurityPolicy{}
}

func (c podSecurityPolicyFactory) List() runtime.Object {
	return &PodSecurityPolicyList{}
}

func (s *podSecurityPolicyClient) Controller() PodSecurityPolicyController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podSecurityPolicyControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodSecurityPolicyGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podSecurityPolicyController{
		GenericController: genericController,
	}

	s.client.podSecurityPolicyControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podSecurityPolicyClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodSecurityPolicyController
}

func (s *podSecurityPolicyClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyClient) Create(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Get(name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Update(o *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityPolicyClient) List(opts metav1.ListOptions) (*PodSecurityPolicyList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodSecurityPolicyList), err
}

func (s *podSecurityPolicyClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*PodSecurityPolicyList), err
}

func (s *podSecurityPolicyClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyClient) Patch(o *v1beta1.PodSecurityPolicy, patchType types.PatchType, data []byte, subresources ...string) (*v1beta1.PodSecurityPolicy, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1beta1.PodSecurityPolicy), err
}

func (s *podSecurityPolicyClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyClient) AddHandler(ctx context.Context, name string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyClient) AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyLifecycle) {
	sync := NewPodSecurityPolicyLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type PodSecurityPolicyIndexer func(obj *v1beta1.PodSecurityPolicy) ([]string, error)

type PodSecurityPolicyClientCache interface {
	Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error)
	List(namespace string, selector labels.Selector) ([]*v1beta1.PodSecurityPolicy, error)

	Index(name string, indexer PodSecurityPolicyIndexer)
	GetIndexed(name, key string) ([]*v1beta1.PodSecurityPolicy, error)
}

type PodSecurityPolicyClient interface {
	Create(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error)
	Update(*v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() PodSecurityPolicyClientCache

	OnCreate(ctx context.Context, name string, sync PodSecurityPolicyChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync PodSecurityPolicyChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync PodSecurityPolicyChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() PodSecurityPolicyInterface
}

type podSecurityPolicyClientCache struct {
	client *podSecurityPolicyClient2
}

type podSecurityPolicyClient2 struct {
	iface      PodSecurityPolicyInterface
	controller PodSecurityPolicyController
}

func (n *podSecurityPolicyClient2) Interface() PodSecurityPolicyInterface {
	return n.iface
}

func (n *podSecurityPolicyClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *podSecurityPolicyClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *podSecurityPolicyClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *podSecurityPolicyClient2) Create(obj *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	return n.iface.Create(obj)
}

func (n *podSecurityPolicyClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1beta1.PodSecurityPolicy, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *podSecurityPolicyClient2) Update(obj *v1beta1.PodSecurityPolicy) (*v1beta1.PodSecurityPolicy, error) {
	return n.iface.Update(obj)
}

func (n *podSecurityPolicyClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *podSecurityPolicyClient2) List(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyList, error) {
	return n.iface.List(opts)
}

func (n *podSecurityPolicyClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *podSecurityPolicyClientCache) Get(namespace, name string) (*v1beta1.PodSecurityPolicy, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *podSecurityPolicyClientCache) List(namespace string, selector labels.Selector) ([]*v1beta1.PodSecurityPolicy, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *podSecurityPolicyClient2) Cache() PodSecurityPolicyClientCache {
	n.loadController()
	return &podSecurityPolicyClientCache{
		client: n,
	}
}

func (n *podSecurityPolicyClient2) OnCreate(ctx context.Context, name string, sync PodSecurityPolicyChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &podSecurityPolicyLifecycleDelegate{create: sync})
}

func (n *podSecurityPolicyClient2) OnChange(ctx context.Context, name string, sync PodSecurityPolicyChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &podSecurityPolicyLifecycleDelegate{update: sync})
}

func (n *podSecurityPolicyClient2) OnRemove(ctx context.Context, name string, sync PodSecurityPolicyChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &podSecurityPolicyLifecycleDelegate{remove: sync})
}

func (n *podSecurityPolicyClientCache) Index(name string, indexer PodSecurityPolicyIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *podSecurityPolicyClientCache) GetIndexed(name, key string) ([]*v1beta1.PodSecurityPolicy, error) {
	var result []*v1beta1.PodSecurityPolicy
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1beta1.PodSecurityPolicy); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *podSecurityPolicyClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type podSecurityPolicyLifecycleDelegate struct {
	create PodSecurityPolicyChangeHandlerFunc
	update PodSecurityPolicyChangeHandlerFunc
	remove PodSecurityPolicyChangeHandlerFunc
}

func (n *podSecurityPolicyLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *podSecurityPolicyLifecycleDelegate) Create(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *podSecurityPolicyLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *podSecurityPolicyLifecycleDelegate) Remove(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *podSecurityPolicyLifecycleDelegate) Updated(obj *v1beta1.PodSecurityPolicy) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
