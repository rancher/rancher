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
	CloudCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CloudCredential",
	}
	CloudCredentialResource = metav1.APIResource{
		Name:         "cloudcredentials",
		SingularName: "cloudcredential",
		Namespaced:   true,

		Kind: CloudCredentialGroupVersionKind.Kind,
	}

	CloudCredentialGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "cloudcredentials",
	}
)

func init() {
	resource.Put(CloudCredentialGroupVersionResource)
}

func NewCloudCredential(namespace, name string, obj CloudCredential) *CloudCredential {
	obj.APIVersion, obj.Kind = CloudCredentialGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CloudCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudCredential `json:"items"`
}

type CloudCredentialHandlerFunc func(key string, obj *CloudCredential) (runtime.Object, error)

type CloudCredentialChangeHandlerFunc func(obj *CloudCredential) (runtime.Object, error)

type CloudCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*CloudCredential, err error)
	Get(namespace, name string) (*CloudCredential, error)
}

type CloudCredentialController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CloudCredentialLister
	AddHandler(ctx context.Context, name string, handler CloudCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CloudCredentialHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CloudCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CloudCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CloudCredentialInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*CloudCredential) (*CloudCredential, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CloudCredential, error)
	Get(name string, opts metav1.GetOptions) (*CloudCredential, error)
	Update(*CloudCredential) (*CloudCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CloudCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CloudCredentialController
	AddHandler(ctx context.Context, name string, sync CloudCredentialHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CloudCredentialHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CloudCredentialLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CloudCredentialLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CloudCredentialHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CloudCredentialHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CloudCredentialLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CloudCredentialLifecycle)
}

type cloudCredentialLister struct {
	controller *cloudCredentialController
}

func (l *cloudCredentialLister) List(namespace string, selector labels.Selector) (ret []*CloudCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*CloudCredential))
	})
	return
}

func (l *cloudCredentialLister) Get(namespace, name string) (*CloudCredential, error) {
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
			Group:    CloudCredentialGroupVersionKind.Group,
			Resource: "cloudCredential",
		}, key)
	}
	return obj.(*CloudCredential), nil
}

type cloudCredentialController struct {
	controller.GenericController
}

func (c *cloudCredentialController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cloudCredentialController) Lister() CloudCredentialLister {
	return &cloudCredentialLister{
		controller: c,
	}
}

func (c *cloudCredentialController) AddHandler(ctx context.Context, name string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CloudCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cloudCredentialController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CloudCredential); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cloudCredentialController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CloudCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cloudCredentialController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CloudCredentialHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*CloudCredential); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type cloudCredentialFactory struct {
}

func (c cloudCredentialFactory) Object() runtime.Object {
	return &CloudCredential{}
}

func (c cloudCredentialFactory) List() runtime.Object {
	return &CloudCredentialList{}
}

func (s *cloudCredentialClient) Controller() CloudCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.cloudCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CloudCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &cloudCredentialController{
		GenericController: genericController,
	}

	s.client.cloudCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type cloudCredentialClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CloudCredentialController
}

func (s *cloudCredentialClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cloudCredentialClient) Create(o *CloudCredential) (*CloudCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) Get(name string, opts metav1.GetOptions) (*CloudCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*CloudCredential, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) Update(o *CloudCredential) (*CloudCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cloudCredentialClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cloudCredentialClient) List(opts metav1.ListOptions) (*CloudCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CloudCredentialList), err
}

func (s *cloudCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cloudCredentialClient) Patch(o *CloudCredential, patchType types.PatchType, data []byte, subresources ...string) (*CloudCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*CloudCredential), err
}

func (s *cloudCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cloudCredentialClient) AddHandler(ctx context.Context, name string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cloudCredentialClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cloudCredentialClient) AddLifecycle(ctx context.Context, name string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cloudCredentialClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cloudCredentialClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CloudCredentialHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cloudCredentialClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CloudCredentialLifecycle) {
	sync := NewCloudCredentialLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type CloudCredentialIndexer func(obj *CloudCredential) ([]string, error)

type CloudCredentialClientCache interface {
	Get(namespace, name string) (*CloudCredential, error)
	List(namespace string, selector labels.Selector) ([]*CloudCredential, error)

	Index(name string, indexer CloudCredentialIndexer)
	GetIndexed(name, key string) ([]*CloudCredential, error)
}

type CloudCredentialClient interface {
	Create(*CloudCredential) (*CloudCredential, error)
	Get(namespace, name string, opts metav1.GetOptions) (*CloudCredential, error)
	Update(*CloudCredential) (*CloudCredential, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*CloudCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() CloudCredentialClientCache

	OnCreate(ctx context.Context, name string, sync CloudCredentialChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync CloudCredentialChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync CloudCredentialChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() CloudCredentialInterface
}

type cloudCredentialClientCache struct {
	client *cloudCredentialClient2
}

type cloudCredentialClient2 struct {
	iface      CloudCredentialInterface
	controller CloudCredentialController
}

func (n *cloudCredentialClient2) Interface() CloudCredentialInterface {
	return n.iface
}

func (n *cloudCredentialClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *cloudCredentialClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *cloudCredentialClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *cloudCredentialClient2) Create(obj *CloudCredential) (*CloudCredential, error) {
	return n.iface.Create(obj)
}

func (n *cloudCredentialClient2) Get(namespace, name string, opts metav1.GetOptions) (*CloudCredential, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *cloudCredentialClient2) Update(obj *CloudCredential) (*CloudCredential, error) {
	return n.iface.Update(obj)
}

func (n *cloudCredentialClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *cloudCredentialClient2) List(namespace string, opts metav1.ListOptions) (*CloudCredentialList, error) {
	return n.iface.List(opts)
}

func (n *cloudCredentialClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *cloudCredentialClientCache) Get(namespace, name string) (*CloudCredential, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *cloudCredentialClientCache) List(namespace string, selector labels.Selector) ([]*CloudCredential, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *cloudCredentialClient2) Cache() CloudCredentialClientCache {
	n.loadController()
	return &cloudCredentialClientCache{
		client: n,
	}
}

func (n *cloudCredentialClient2) OnCreate(ctx context.Context, name string, sync CloudCredentialChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &cloudCredentialLifecycleDelegate{create: sync})
}

func (n *cloudCredentialClient2) OnChange(ctx context.Context, name string, sync CloudCredentialChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &cloudCredentialLifecycleDelegate{update: sync})
}

func (n *cloudCredentialClient2) OnRemove(ctx context.Context, name string, sync CloudCredentialChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &cloudCredentialLifecycleDelegate{remove: sync})
}

func (n *cloudCredentialClientCache) Index(name string, indexer CloudCredentialIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*CloudCredential); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *cloudCredentialClientCache) GetIndexed(name, key string) ([]*CloudCredential, error) {
	var result []*CloudCredential
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*CloudCredential); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *cloudCredentialClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type cloudCredentialLifecycleDelegate struct {
	create CloudCredentialChangeHandlerFunc
	update CloudCredentialChangeHandlerFunc
	remove CloudCredentialChangeHandlerFunc
}

func (n *cloudCredentialLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *cloudCredentialLifecycleDelegate) Create(obj *CloudCredential) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *cloudCredentialLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *cloudCredentialLifecycleDelegate) Remove(obj *CloudCredential) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *cloudCredentialLifecycleDelegate) Updated(obj *CloudCredential) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
