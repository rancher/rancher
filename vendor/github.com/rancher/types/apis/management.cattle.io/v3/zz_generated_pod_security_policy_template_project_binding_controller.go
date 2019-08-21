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
	PodSecurityPolicyTemplateProjectBindingGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicyTemplateProjectBinding",
	}
	PodSecurityPolicyTemplateProjectBindingResource = metav1.APIResource{
		Name:         "podsecuritypolicytemplateprojectbindings",
		SingularName: "podsecuritypolicytemplateprojectbinding",
		Namespaced:   true,

		Kind: PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Kind,
	}

	PodSecurityPolicyTemplateProjectBindingGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "podsecuritypolicytemplateprojectbindings",
	}
)

func init() {
	resource.Put(PodSecurityPolicyTemplateProjectBindingGroupVersionResource)
}

func NewPodSecurityPolicyTemplateProjectBinding(namespace, name string, obj PodSecurityPolicyTemplateProjectBinding) *PodSecurityPolicyTemplateProjectBinding {
	obj.APIVersion, obj.Kind = PodSecurityPolicyTemplateProjectBindingGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodSecurityPolicyTemplateProjectBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodSecurityPolicyTemplateProjectBinding `json:"items"`
}

type PodSecurityPolicyTemplateProjectBindingHandlerFunc func(key string, obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error)

type PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc func(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error)

type PodSecurityPolicyTemplateProjectBindingLister interface {
	List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplateProjectBinding, err error)
	Get(namespace, name string) (*PodSecurityPolicyTemplateProjectBinding, error)
}

type PodSecurityPolicyTemplateProjectBindingController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyTemplateProjectBindingLister
	AddHandler(ctx context.Context, name string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodSecurityPolicyTemplateProjectBindingInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error)
	Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error)
	Update(*PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateProjectBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyTemplateProjectBindingController
	AddHandler(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle)
}

type podSecurityPolicyTemplateProjectBindingLister struct {
	controller *podSecurityPolicyTemplateProjectBindingController
}

func (l *podSecurityPolicyTemplateProjectBindingLister) List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplateProjectBinding, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PodSecurityPolicyTemplateProjectBinding))
	})
	return
}

func (l *podSecurityPolicyTemplateProjectBindingLister) Get(namespace, name string) (*PodSecurityPolicyTemplateProjectBinding, error) {
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
			Group:    PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Group,
			Resource: "podSecurityPolicyTemplateProjectBinding",
		}, key)
	}
	return obj.(*PodSecurityPolicyTemplateProjectBinding), nil
}

type podSecurityPolicyTemplateProjectBindingController struct {
	controller.GenericController
}

func (c *podSecurityPolicyTemplateProjectBindingController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podSecurityPolicyTemplateProjectBindingController) Lister() PodSecurityPolicyTemplateProjectBindingLister {
	return &podSecurityPolicyTemplateProjectBindingLister{
		controller: c,
	}
}

func (c *podSecurityPolicyTemplateProjectBindingController) AddHandler(ctx context.Context, name string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplateProjectBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateProjectBindingController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplateProjectBinding); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateProjectBindingController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplateProjectBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateProjectBindingController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplateProjectBinding); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podSecurityPolicyTemplateProjectBindingFactory struct {
}

func (c podSecurityPolicyTemplateProjectBindingFactory) Object() runtime.Object {
	return &PodSecurityPolicyTemplateProjectBinding{}
}

func (c podSecurityPolicyTemplateProjectBindingFactory) List() runtime.Object {
	return &PodSecurityPolicyTemplateProjectBindingList{}
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Controller() PodSecurityPolicyTemplateProjectBindingController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podSecurityPolicyTemplateProjectBindingControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodSecurityPolicyTemplateProjectBindingGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podSecurityPolicyTemplateProjectBindingController{
		GenericController: genericController,
	}

	s.client.podSecurityPolicyTemplateProjectBindingControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podSecurityPolicyTemplateProjectBindingClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodSecurityPolicyTemplateProjectBindingController
}

func (s *podSecurityPolicyTemplateProjectBindingClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Create(o *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Update(o *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateProjectBindingList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodSecurityPolicyTemplateProjectBindingList), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyTemplateProjectBindingClient) Patch(o *PodSecurityPolicyTemplateProjectBinding, patchType types.PatchType, data []byte, subresources ...string) (*PodSecurityPolicyTemplateProjectBinding, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*PodSecurityPolicyTemplateProjectBinding), err
}

func (s *podSecurityPolicyTemplateProjectBindingClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddHandler(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle) {
	sync := NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle) {
	sync := NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyTemplateProjectBindingHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle) {
	sync := NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateProjectBindingClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyTemplateProjectBindingLifecycle) {
	sync := NewPodSecurityPolicyTemplateProjectBindingLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type PodSecurityPolicyTemplateProjectBindingIndexer func(obj *PodSecurityPolicyTemplateProjectBinding) ([]string, error)

type PodSecurityPolicyTemplateProjectBindingClientCache interface {
	Get(namespace, name string) (*PodSecurityPolicyTemplateProjectBinding, error)
	List(namespace string, selector labels.Selector) ([]*PodSecurityPolicyTemplateProjectBinding, error)

	Index(name string, indexer PodSecurityPolicyTemplateProjectBindingIndexer)
	GetIndexed(name, key string) ([]*PodSecurityPolicyTemplateProjectBinding, error)
}

type PodSecurityPolicyTemplateProjectBindingClient interface {
	Create(*PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Get(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error)
	Update(*PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyTemplateProjectBindingList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() PodSecurityPolicyTemplateProjectBindingClientCache

	OnCreate(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() PodSecurityPolicyTemplateProjectBindingInterface
}

type podSecurityPolicyTemplateProjectBindingClientCache struct {
	client *podSecurityPolicyTemplateProjectBindingClient2
}

type podSecurityPolicyTemplateProjectBindingClient2 struct {
	iface      PodSecurityPolicyTemplateProjectBindingInterface
	controller PodSecurityPolicyTemplateProjectBindingController
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Interface() PodSecurityPolicyTemplateProjectBindingInterface {
	return n.iface
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Create(obj *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	return n.iface.Create(obj)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Get(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplateProjectBinding, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Update(obj *PodSecurityPolicyTemplateProjectBinding) (*PodSecurityPolicyTemplateProjectBinding, error) {
	return n.iface.Update(obj)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) List(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyTemplateProjectBindingList, error) {
	return n.iface.List(opts)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *podSecurityPolicyTemplateProjectBindingClientCache) Get(namespace, name string) (*PodSecurityPolicyTemplateProjectBinding, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *podSecurityPolicyTemplateProjectBindingClientCache) List(namespace string, selector labels.Selector) ([]*PodSecurityPolicyTemplateProjectBinding, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) Cache() PodSecurityPolicyTemplateProjectBindingClientCache {
	n.loadController()
	return &podSecurityPolicyTemplateProjectBindingClientCache{
		client: n,
	}
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) OnCreate(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &podSecurityPolicyTemplateProjectBindingLifecycleDelegate{create: sync})
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) OnChange(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &podSecurityPolicyTemplateProjectBindingLifecycleDelegate{update: sync})
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) OnRemove(ctx context.Context, name string, sync PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &podSecurityPolicyTemplateProjectBindingLifecycleDelegate{remove: sync})
}

func (n *podSecurityPolicyTemplateProjectBindingClientCache) Index(name string, indexer PodSecurityPolicyTemplateProjectBindingIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*PodSecurityPolicyTemplateProjectBinding); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *podSecurityPolicyTemplateProjectBindingClientCache) GetIndexed(name, key string) ([]*PodSecurityPolicyTemplateProjectBinding, error) {
	var result []*PodSecurityPolicyTemplateProjectBinding
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*PodSecurityPolicyTemplateProjectBinding); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *podSecurityPolicyTemplateProjectBindingClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type podSecurityPolicyTemplateProjectBindingLifecycleDelegate struct {
	create PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc
	update PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc
	remove PodSecurityPolicyTemplateProjectBindingChangeHandlerFunc
}

func (n *podSecurityPolicyTemplateProjectBindingLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *podSecurityPolicyTemplateProjectBindingLifecycleDelegate) Create(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *podSecurityPolicyTemplateProjectBindingLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *podSecurityPolicyTemplateProjectBindingLifecycleDelegate) Remove(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *podSecurityPolicyTemplateProjectBindingLifecycleDelegate) Updated(obj *PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
