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
	CertificateGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "Certificate",
	}
	CertificateResource = metav1.APIResource{
		Name:         "certificates",
		SingularName: "certificate",
		Namespaced:   true,

		Kind: CertificateGroupVersionKind.Kind,
	}

	CertificateGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "certificates",
	}
)

func init() {
	resource.Put(CertificateGroupVersionResource)
}

func NewCertificate(namespace, name string, obj Certificate) *Certificate {
	obj.APIVersion, obj.Kind = CertificateGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

type CertificateHandlerFunc func(key string, obj *Certificate) (runtime.Object, error)

type CertificateChangeHandlerFunc func(obj *Certificate) (runtime.Object, error)

type CertificateLister interface {
	List(namespace string, selector labels.Selector) (ret []*Certificate, err error)
	Get(namespace, name string) (*Certificate, error)
}

type CertificateController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CertificateLister
	AddHandler(ctx context.Context, name string, handler CertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CertificateHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CertificateHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CertificateInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*Certificate) (*Certificate, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Certificate, error)
	Get(name string, opts metav1.GetOptions) (*Certificate, error)
	Update(*Certificate) (*Certificate, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CertificateList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*CertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CertificateController
	AddHandler(ctx context.Context, name string, sync CertificateHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CertificateHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CertificateLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CertificateLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CertificateHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CertificateHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CertificateLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CertificateLifecycle)
}

type certificateLister struct {
	controller *certificateController
}

func (l *certificateLister) List(namespace string, selector labels.Selector) (ret []*Certificate, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*Certificate))
	})
	return
}

func (l *certificateLister) Get(namespace, name string) (*Certificate, error) {
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
			Group:    CertificateGroupVersionKind.Group,
			Resource: "certificate",
		}, key)
	}
	return obj.(*Certificate), nil
}

type certificateController struct {
	controller.GenericController
}

func (c *certificateController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *certificateController) Lister() CertificateLister {
	return &certificateLister{
		controller: c,
	}
}

func (c *certificateController) AddHandler(ctx context.Context, name string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Certificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *certificateController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Certificate); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *certificateController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Certificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *certificateController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CertificateHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*Certificate); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type certificateFactory struct {
}

func (c certificateFactory) Object() runtime.Object {
	return &Certificate{}
}

func (c certificateFactory) List() runtime.Object {
	return &CertificateList{}
}

func (s *certificateClient) Controller() CertificateController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.certificateControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CertificateGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &certificateController{
		GenericController: genericController,
	}

	s.client.certificateControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type certificateClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CertificateController
}

func (s *certificateClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *certificateClient) Create(o *Certificate) (*Certificate, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*Certificate), err
}

func (s *certificateClient) Get(name string, opts metav1.GetOptions) (*Certificate, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*Certificate), err
}

func (s *certificateClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*Certificate, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*Certificate), err
}

func (s *certificateClient) Update(o *Certificate) (*Certificate, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*Certificate), err
}

func (s *certificateClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *certificateClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *certificateClient) List(opts metav1.ListOptions) (*CertificateList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CertificateList), err
}

func (s *certificateClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*CertificateList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*CertificateList), err
}

func (s *certificateClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *certificateClient) Patch(o *Certificate, patchType types.PatchType, data []byte, subresources ...string) (*Certificate, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*Certificate), err
}

func (s *certificateClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *certificateClient) AddHandler(ctx context.Context, name string, sync CertificateHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *certificateClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CertificateHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *certificateClient) AddLifecycle(ctx context.Context, name string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *certificateClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *certificateClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CertificateHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CertificateHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *certificateClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CertificateLifecycle) {
	sync := NewCertificateLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type CertificateIndexer func(obj *Certificate) ([]string, error)

type CertificateClientCache interface {
	Get(namespace, name string) (*Certificate, error)
	List(namespace string, selector labels.Selector) ([]*Certificate, error)

	Index(name string, indexer CertificateIndexer)
	GetIndexed(name, key string) ([]*Certificate, error)
}

type CertificateClient interface {
	Create(*Certificate) (*Certificate, error)
	Get(namespace, name string, opts metav1.GetOptions) (*Certificate, error)
	Update(*Certificate) (*Certificate, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*CertificateList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() CertificateClientCache

	OnCreate(ctx context.Context, name string, sync CertificateChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync CertificateChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync CertificateChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() CertificateInterface
}

type certificateClientCache struct {
	client *certificateClient2
}

type certificateClient2 struct {
	iface      CertificateInterface
	controller CertificateController
}

func (n *certificateClient2) Interface() CertificateInterface {
	return n.iface
}

func (n *certificateClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *certificateClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *certificateClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *certificateClient2) Create(obj *Certificate) (*Certificate, error) {
	return n.iface.Create(obj)
}

func (n *certificateClient2) Get(namespace, name string, opts metav1.GetOptions) (*Certificate, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *certificateClient2) Update(obj *Certificate) (*Certificate, error) {
	return n.iface.Update(obj)
}

func (n *certificateClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *certificateClient2) List(namespace string, opts metav1.ListOptions) (*CertificateList, error) {
	return n.iface.List(opts)
}

func (n *certificateClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *certificateClientCache) Get(namespace, name string) (*Certificate, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *certificateClientCache) List(namespace string, selector labels.Selector) ([]*Certificate, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *certificateClient2) Cache() CertificateClientCache {
	n.loadController()
	return &certificateClientCache{
		client: n,
	}
}

func (n *certificateClient2) OnCreate(ctx context.Context, name string, sync CertificateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &certificateLifecycleDelegate{create: sync})
}

func (n *certificateClient2) OnChange(ctx context.Context, name string, sync CertificateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &certificateLifecycleDelegate{update: sync})
}

func (n *certificateClient2) OnRemove(ctx context.Context, name string, sync CertificateChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &certificateLifecycleDelegate{remove: sync})
}

func (n *certificateClientCache) Index(name string, indexer CertificateIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*Certificate); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *certificateClientCache) GetIndexed(name, key string) ([]*Certificate, error) {
	var result []*Certificate
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*Certificate); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *certificateClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type certificateLifecycleDelegate struct {
	create CertificateChangeHandlerFunc
	update CertificateChangeHandlerFunc
	remove CertificateChangeHandlerFunc
}

func (n *certificateLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *certificateLifecycleDelegate) Create(obj *Certificate) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *certificateLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *certificateLifecycleDelegate) Remove(obj *Certificate) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *certificateLifecycleDelegate) Updated(obj *Certificate) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
