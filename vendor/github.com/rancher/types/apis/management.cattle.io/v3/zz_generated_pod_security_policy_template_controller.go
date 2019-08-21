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
	PodSecurityPolicyTemplateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "PodSecurityPolicyTemplate",
	}
	PodSecurityPolicyTemplateResource = metav1.APIResource{
		Name:         "podsecuritypolicytemplates",
		SingularName: "podsecuritypolicytemplate",
		Namespaced:   false,
		Kind:         PodSecurityPolicyTemplateGroupVersionKind.Kind,
	}

	PodSecurityPolicyTemplateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "podsecuritypolicytemplates",
	}
)

func init() {
	resource.Put(PodSecurityPolicyTemplateGroupVersionResource)
}

func NewPodSecurityPolicyTemplate(namespace, name string, obj PodSecurityPolicyTemplate) *PodSecurityPolicyTemplate {
	obj.APIVersion, obj.Kind = PodSecurityPolicyTemplateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type PodSecurityPolicyTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodSecurityPolicyTemplate `json:"items"`
}

type PodSecurityPolicyTemplateHandlerFunc func(key string, obj *PodSecurityPolicyTemplate) (runtime.Object, error)

type PodSecurityPolicyTemplateChangeHandlerFunc func(obj *PodSecurityPolicyTemplate) (runtime.Object, error)

type PodSecurityPolicyTemplateLister interface {
	List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplate, err error)
	Get(namespace, name string) (*PodSecurityPolicyTemplate, error)
}

type PodSecurityPolicyTemplateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() PodSecurityPolicyTemplateLister
	AddHandler(ctx context.Context, name string, handler PodSecurityPolicyTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler PodSecurityPolicyTemplateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type PodSecurityPolicyTemplateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error)
	Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error)
	Update(*PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() PodSecurityPolicyTemplateController
	AddHandler(ctx context.Context, name string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyTemplateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyTemplateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle)
}

type podSecurityPolicyTemplateLister struct {
	controller *podSecurityPolicyTemplateController
}

func (l *podSecurityPolicyTemplateLister) List(namespace string, selector labels.Selector) (ret []*PodSecurityPolicyTemplate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*PodSecurityPolicyTemplate))
	})
	return
}

func (l *podSecurityPolicyTemplateLister) Get(namespace, name string) (*PodSecurityPolicyTemplate, error) {
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
			Group:    PodSecurityPolicyTemplateGroupVersionKind.Group,
			Resource: "podSecurityPolicyTemplate",
		}, key)
	}
	return obj.(*PodSecurityPolicyTemplate), nil
}

type podSecurityPolicyTemplateController struct {
	controller.GenericController
}

func (c *podSecurityPolicyTemplateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *podSecurityPolicyTemplateController) Lister() PodSecurityPolicyTemplateLister {
	return &podSecurityPolicyTemplateLister{
		controller: c,
	}
}

func (c *podSecurityPolicyTemplateController) AddHandler(ctx context.Context, name string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *podSecurityPolicyTemplateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler PodSecurityPolicyTemplateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*PodSecurityPolicyTemplate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type podSecurityPolicyTemplateFactory struct {
}

func (c podSecurityPolicyTemplateFactory) Object() runtime.Object {
	return &PodSecurityPolicyTemplate{}
}

func (c podSecurityPolicyTemplateFactory) List() runtime.Object {
	return &PodSecurityPolicyTemplateList{}
}

func (s *podSecurityPolicyTemplateClient) Controller() PodSecurityPolicyTemplateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.podSecurityPolicyTemplateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(PodSecurityPolicyTemplateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &podSecurityPolicyTemplateController{
		GenericController: genericController,
	}

	s.client.podSecurityPolicyTemplateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type podSecurityPolicyTemplateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   PodSecurityPolicyTemplateController
}

func (s *podSecurityPolicyTemplateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *podSecurityPolicyTemplateClient) Create(o *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Get(name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Update(o *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *podSecurityPolicyTemplateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *podSecurityPolicyTemplateClient) List(opts metav1.ListOptions) (*PodSecurityPolicyTemplateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*PodSecurityPolicyTemplateList), err
}

func (s *podSecurityPolicyTemplateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *podSecurityPolicyTemplateClient) Patch(o *PodSecurityPolicyTemplate, patchType types.PatchType, data []byte, subresources ...string) (*PodSecurityPolicyTemplate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*PodSecurityPolicyTemplate), err
}

func (s *podSecurityPolicyTemplateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *podSecurityPolicyTemplateClient) AddHandler(ctx context.Context, name string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddLifecycle(ctx context.Context, name string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync PodSecurityPolicyTemplateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *podSecurityPolicyTemplateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle PodSecurityPolicyTemplateLifecycle) {
	sync := NewPodSecurityPolicyTemplateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type PodSecurityPolicyTemplateIndexer func(obj *PodSecurityPolicyTemplate) ([]string, error)

type PodSecurityPolicyTemplateClientCache interface {
	Get(namespace, name string) (*PodSecurityPolicyTemplate, error)
	List(namespace string, selector labels.Selector) ([]*PodSecurityPolicyTemplate, error)

	Index(name string, indexer PodSecurityPolicyTemplateIndexer)
	GetIndexed(name, key string) ([]*PodSecurityPolicyTemplate, error)
}

type PodSecurityPolicyTemplateClient interface {
	Create(*PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error)
	Update(*PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyTemplateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() PodSecurityPolicyTemplateClientCache

	OnCreate(ctx context.Context, name string, sync PodSecurityPolicyTemplateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync PodSecurityPolicyTemplateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync PodSecurityPolicyTemplateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() PodSecurityPolicyTemplateInterface
}

type podSecurityPolicyTemplateClientCache struct {
	client *podSecurityPolicyTemplateClient2
}

type podSecurityPolicyTemplateClient2 struct {
	iface      PodSecurityPolicyTemplateInterface
	controller PodSecurityPolicyTemplateController
}

func (n *podSecurityPolicyTemplateClient2) Interface() PodSecurityPolicyTemplateInterface {
	return n.iface
}

func (n *podSecurityPolicyTemplateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *podSecurityPolicyTemplateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *podSecurityPolicyTemplateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *podSecurityPolicyTemplateClient2) Create(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	return n.iface.Create(obj)
}

func (n *podSecurityPolicyTemplateClient2) Get(namespace, name string, opts metav1.GetOptions) (*PodSecurityPolicyTemplate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *podSecurityPolicyTemplateClient2) Update(obj *PodSecurityPolicyTemplate) (*PodSecurityPolicyTemplate, error) {
	return n.iface.Update(obj)
}

func (n *podSecurityPolicyTemplateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *podSecurityPolicyTemplateClient2) List(namespace string, opts metav1.ListOptions) (*PodSecurityPolicyTemplateList, error) {
	return n.iface.List(opts)
}

func (n *podSecurityPolicyTemplateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *podSecurityPolicyTemplateClientCache) Get(namespace, name string) (*PodSecurityPolicyTemplate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *podSecurityPolicyTemplateClientCache) List(namespace string, selector labels.Selector) ([]*PodSecurityPolicyTemplate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *podSecurityPolicyTemplateClient2) Cache() PodSecurityPolicyTemplateClientCache {
	n.loadController()
	return &podSecurityPolicyTemplateClientCache{
		client: n,
	}
}

func (n *podSecurityPolicyTemplateClient2) OnCreate(ctx context.Context, name string, sync PodSecurityPolicyTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &podSecurityPolicyTemplateLifecycleDelegate{create: sync})
}

func (n *podSecurityPolicyTemplateClient2) OnChange(ctx context.Context, name string, sync PodSecurityPolicyTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &podSecurityPolicyTemplateLifecycleDelegate{update: sync})
}

func (n *podSecurityPolicyTemplateClient2) OnRemove(ctx context.Context, name string, sync PodSecurityPolicyTemplateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &podSecurityPolicyTemplateLifecycleDelegate{remove: sync})
}

func (n *podSecurityPolicyTemplateClientCache) Index(name string, indexer PodSecurityPolicyTemplateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*PodSecurityPolicyTemplate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *podSecurityPolicyTemplateClientCache) GetIndexed(name, key string) ([]*PodSecurityPolicyTemplate, error) {
	var result []*PodSecurityPolicyTemplate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*PodSecurityPolicyTemplate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *podSecurityPolicyTemplateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type podSecurityPolicyTemplateLifecycleDelegate struct {
	create PodSecurityPolicyTemplateChangeHandlerFunc
	update PodSecurityPolicyTemplateChangeHandlerFunc
	remove PodSecurityPolicyTemplateChangeHandlerFunc
}

func (n *podSecurityPolicyTemplateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *podSecurityPolicyTemplateLifecycleDelegate) Create(obj *PodSecurityPolicyTemplate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *podSecurityPolicyTemplateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *podSecurityPolicyTemplateLifecycleDelegate) Remove(obj *PodSecurityPolicyTemplate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *podSecurityPolicyTemplateLifecycleDelegate) Updated(obj *PodSecurityPolicyTemplate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
