package v1

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
	v1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	APIServiceGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "APIService",
	}
	APIServiceResource = metav1.APIResource{
		Name:         "apiservices",
		SingularName: "apiservice",
		Namespaced:   false,
		Kind:         APIServiceGroupVersionKind.Kind,
	}

	APIServiceGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "apiservices",
	}
)

func init() {
	resource.Put(APIServiceGroupVersionResource)
}

func NewAPIService(namespace, name string, obj v1.APIService) *v1.APIService {
	obj.APIVersion, obj.Kind = APIServiceGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type APIServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1.APIService `json:"items"`
}

type APIServiceHandlerFunc func(key string, obj *v1.APIService) (runtime.Object, error)

type APIServiceChangeHandlerFunc func(obj *v1.APIService) (runtime.Object, error)

type APIServiceLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1.APIService, err error)
	Get(namespace, name string) (*v1.APIService, error)
}

type APIServiceController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() APIServiceLister
	AddHandler(ctx context.Context, name string, handler APIServiceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync APIServiceHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler APIServiceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler APIServiceHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type APIServiceInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1.APIService) (*v1.APIService, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.APIService, error)
	Get(name string, opts metav1.GetOptions) (*v1.APIService, error)
	Update(*v1.APIService) (*v1.APIService, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*APIServiceList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*APIServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() APIServiceController
	AddHandler(ctx context.Context, name string, sync APIServiceHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync APIServiceHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle APIServiceLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle APIServiceLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync APIServiceHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync APIServiceHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle APIServiceLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle APIServiceLifecycle)
}

type apiServiceLister struct {
	controller *apiServiceController
}

func (l *apiServiceLister) List(namespace string, selector labels.Selector) (ret []*v1.APIService, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1.APIService))
	})
	return
}

func (l *apiServiceLister) Get(namespace, name string) (*v1.APIService, error) {
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
			Group:    APIServiceGroupVersionKind.Group,
			Resource: "apiService",
		}, key)
	}
	return obj.(*v1.APIService), nil
}

type apiServiceController struct {
	controller.GenericController
}

func (c *apiServiceController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *apiServiceController) Lister() APIServiceLister {
	return &apiServiceLister{
		controller: c,
	}
}

func (c *apiServiceController) AddHandler(ctx context.Context, name string, handler APIServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.APIService); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *apiServiceController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler APIServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.APIService); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *apiServiceController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler APIServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.APIService); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *apiServiceController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler APIServiceHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1.APIService); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type apiServiceFactory struct {
}

func (c apiServiceFactory) Object() runtime.Object {
	return &v1.APIService{}
}

func (c apiServiceFactory) List() runtime.Object {
	return &APIServiceList{}
}

func (s *apiServiceClient) Controller() APIServiceController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.apiServiceControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(APIServiceGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &apiServiceController{
		GenericController: genericController,
	}

	s.client.apiServiceControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type apiServiceClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   APIServiceController
}

func (s *apiServiceClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *apiServiceClient) Create(o *v1.APIService) (*v1.APIService, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1.APIService), err
}

func (s *apiServiceClient) Get(name string, opts metav1.GetOptions) (*v1.APIService, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1.APIService), err
}

func (s *apiServiceClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1.APIService, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1.APIService), err
}

func (s *apiServiceClient) Update(o *v1.APIService) (*v1.APIService, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1.APIService), err
}

func (s *apiServiceClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *apiServiceClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *apiServiceClient) List(opts metav1.ListOptions) (*APIServiceList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*APIServiceList), err
}

func (s *apiServiceClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*APIServiceList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*APIServiceList), err
}

func (s *apiServiceClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *apiServiceClient) Patch(o *v1.APIService, patchType types.PatchType, data []byte, subresources ...string) (*v1.APIService, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1.APIService), err
}

func (s *apiServiceClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *apiServiceClient) AddHandler(ctx context.Context, name string, sync APIServiceHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *apiServiceClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync APIServiceHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *apiServiceClient) AddLifecycle(ctx context.Context, name string, lifecycle APIServiceLifecycle) {
	sync := NewAPIServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *apiServiceClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle APIServiceLifecycle) {
	sync := NewAPIServiceLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *apiServiceClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync APIServiceHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *apiServiceClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync APIServiceHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *apiServiceClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle APIServiceLifecycle) {
	sync := NewAPIServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *apiServiceClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle APIServiceLifecycle) {
	sync := NewAPIServiceLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type APIServiceIndexer func(obj *v1.APIService) ([]string, error)

type APIServiceClientCache interface {
	Get(namespace, name string) (*v1.APIService, error)
	List(namespace string, selector labels.Selector) ([]*v1.APIService, error)

	Index(name string, indexer APIServiceIndexer)
	GetIndexed(name, key string) ([]*v1.APIService, error)
}

type APIServiceClient interface {
	Create(*v1.APIService) (*v1.APIService, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1.APIService, error)
	Update(*v1.APIService) (*v1.APIService, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*APIServiceList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() APIServiceClientCache

	OnCreate(ctx context.Context, name string, sync APIServiceChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync APIServiceChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync APIServiceChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() APIServiceInterface
}

type apiServiceClientCache struct {
	client *apiServiceClient2
}

type apiServiceClient2 struct {
	iface      APIServiceInterface
	controller APIServiceController
}

func (n *apiServiceClient2) Interface() APIServiceInterface {
	return n.iface
}

func (n *apiServiceClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *apiServiceClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *apiServiceClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *apiServiceClient2) Create(obj *v1.APIService) (*v1.APIService, error) {
	return n.iface.Create(obj)
}

func (n *apiServiceClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1.APIService, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *apiServiceClient2) Update(obj *v1.APIService) (*v1.APIService, error) {
	return n.iface.Update(obj)
}

func (n *apiServiceClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *apiServiceClient2) List(namespace string, opts metav1.ListOptions) (*APIServiceList, error) {
	return n.iface.List(opts)
}

func (n *apiServiceClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *apiServiceClientCache) Get(namespace, name string) (*v1.APIService, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *apiServiceClientCache) List(namespace string, selector labels.Selector) ([]*v1.APIService, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *apiServiceClient2) Cache() APIServiceClientCache {
	n.loadController()
	return &apiServiceClientCache{
		client: n,
	}
}

func (n *apiServiceClient2) OnCreate(ctx context.Context, name string, sync APIServiceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &apiServiceLifecycleDelegate{create: sync})
}

func (n *apiServiceClient2) OnChange(ctx context.Context, name string, sync APIServiceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &apiServiceLifecycleDelegate{update: sync})
}

func (n *apiServiceClient2) OnRemove(ctx context.Context, name string, sync APIServiceChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &apiServiceLifecycleDelegate{remove: sync})
}

func (n *apiServiceClientCache) Index(name string, indexer APIServiceIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1.APIService); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *apiServiceClientCache) GetIndexed(name, key string) ([]*v1.APIService, error) {
	var result []*v1.APIService
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1.APIService); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *apiServiceClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type apiServiceLifecycleDelegate struct {
	create APIServiceChangeHandlerFunc
	update APIServiceChangeHandlerFunc
	remove APIServiceChangeHandlerFunc
}

func (n *apiServiceLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *apiServiceLifecycleDelegate) Create(obj *v1.APIService) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *apiServiceLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *apiServiceLifecycleDelegate) Remove(obj *v1.APIService) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *apiServiceLifecycleDelegate) Updated(obj *v1.APIService) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
