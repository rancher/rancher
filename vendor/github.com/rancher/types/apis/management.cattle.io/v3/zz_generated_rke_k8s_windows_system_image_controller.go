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
	RKEK8sWindowsSystemImageGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "RKEK8sWindowsSystemImage",
	}
	RKEK8sWindowsSystemImageResource = metav1.APIResource{
		Name:         "rkek8swindowssystemimages",
		SingularName: "rkek8swindowssystemimage",
		Namespaced:   true,

		Kind: RKEK8sWindowsSystemImageGroupVersionKind.Kind,
	}

	RKEK8sWindowsSystemImageGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "rkek8swindowssystemimages",
	}
)

func init() {
	resource.Put(RKEK8sWindowsSystemImageGroupVersionResource)
}

func NewRKEK8sWindowsSystemImage(namespace, name string, obj RKEK8sWindowsSystemImage) *RKEK8sWindowsSystemImage {
	obj.APIVersion, obj.Kind = RKEK8sWindowsSystemImageGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type RKEK8sWindowsSystemImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RKEK8sWindowsSystemImage `json:"items"`
}

type RKEK8sWindowsSystemImageHandlerFunc func(key string, obj *RKEK8sWindowsSystemImage) (runtime.Object, error)

type RKEK8sWindowsSystemImageChangeHandlerFunc func(obj *RKEK8sWindowsSystemImage) (runtime.Object, error)

type RKEK8sWindowsSystemImageLister interface {
	List(namespace string, selector labels.Selector) (ret []*RKEK8sWindowsSystemImage, err error)
	Get(namespace, name string) (*RKEK8sWindowsSystemImage, error)
}

type RKEK8sWindowsSystemImageController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() RKEK8sWindowsSystemImageLister
	AddHandler(ctx context.Context, name string, handler RKEK8sWindowsSystemImageHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sWindowsSystemImageHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler RKEK8sWindowsSystemImageHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler RKEK8sWindowsSystemImageHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type RKEK8sWindowsSystemImageInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEK8sWindowsSystemImage, error)
	Get(name string, opts metav1.GetOptions) (*RKEK8sWindowsSystemImage, error)
	Update(*RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*RKEK8sWindowsSystemImageList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() RKEK8sWindowsSystemImageController
	AddHandler(ctx context.Context, name string, sync RKEK8sWindowsSystemImageHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sWindowsSystemImageHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle RKEK8sWindowsSystemImageLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEK8sWindowsSystemImageLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEK8sWindowsSystemImageHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEK8sWindowsSystemImageHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEK8sWindowsSystemImageLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEK8sWindowsSystemImageLifecycle)
}

type rkeK8sWindowsSystemImageLister struct {
	controller *rkeK8sWindowsSystemImageController
}

func (l *rkeK8sWindowsSystemImageLister) List(namespace string, selector labels.Selector) (ret []*RKEK8sWindowsSystemImage, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*RKEK8sWindowsSystemImage))
	})
	return
}

func (l *rkeK8sWindowsSystemImageLister) Get(namespace, name string) (*RKEK8sWindowsSystemImage, error) {
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
			Group:    RKEK8sWindowsSystemImageGroupVersionKind.Group,
			Resource: "rkeK8sWindowsSystemImage",
		}, key)
	}
	return obj.(*RKEK8sWindowsSystemImage), nil
}

type rkeK8sWindowsSystemImageController struct {
	controller.GenericController
}

func (c *rkeK8sWindowsSystemImageController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *rkeK8sWindowsSystemImageController) Lister() RKEK8sWindowsSystemImageLister {
	return &rkeK8sWindowsSystemImageLister{
		controller: c,
	}
}

func (c *rkeK8sWindowsSystemImageController) AddHandler(ctx context.Context, name string, handler RKEK8sWindowsSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sWindowsSystemImage); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sWindowsSystemImageController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler RKEK8sWindowsSystemImageHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sWindowsSystemImage); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sWindowsSystemImageController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler RKEK8sWindowsSystemImageHandlerFunc) {
	resource.PutClusterScoped(RKEK8sWindowsSystemImageGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sWindowsSystemImage); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *rkeK8sWindowsSystemImageController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler RKEK8sWindowsSystemImageHandlerFunc) {
	resource.PutClusterScoped(RKEK8sWindowsSystemImageGroupVersionResource)
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*RKEK8sWindowsSystemImage); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type rkeK8sWindowsSystemImageFactory struct {
}

func (c rkeK8sWindowsSystemImageFactory) Object() runtime.Object {
	return &RKEK8sWindowsSystemImage{}
}

func (c rkeK8sWindowsSystemImageFactory) List() runtime.Object {
	return &RKEK8sWindowsSystemImageList{}
}

func (s *rkeK8sWindowsSystemImageClient) Controller() RKEK8sWindowsSystemImageController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.rkeK8sWindowsSystemImageControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(RKEK8sWindowsSystemImageGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &rkeK8sWindowsSystemImageController{
		GenericController: genericController,
	}

	s.client.rkeK8sWindowsSystemImageControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type rkeK8sWindowsSystemImageClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   RKEK8sWindowsSystemImageController
}

func (s *rkeK8sWindowsSystemImageClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *rkeK8sWindowsSystemImageClient) Create(o *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*RKEK8sWindowsSystemImage), err
}

func (s *rkeK8sWindowsSystemImageClient) Get(name string, opts metav1.GetOptions) (*RKEK8sWindowsSystemImage, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*RKEK8sWindowsSystemImage), err
}

func (s *rkeK8sWindowsSystemImageClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*RKEK8sWindowsSystemImage, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*RKEK8sWindowsSystemImage), err
}

func (s *rkeK8sWindowsSystemImageClient) Update(o *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*RKEK8sWindowsSystemImage), err
}

func (s *rkeK8sWindowsSystemImageClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *rkeK8sWindowsSystemImageClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *rkeK8sWindowsSystemImageClient) List(opts metav1.ListOptions) (*RKEK8sWindowsSystemImageList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*RKEK8sWindowsSystemImageList), err
}

func (s *rkeK8sWindowsSystemImageClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *rkeK8sWindowsSystemImageClient) Patch(o *RKEK8sWindowsSystemImage, patchType types.PatchType, data []byte, subresources ...string) (*RKEK8sWindowsSystemImage, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*RKEK8sWindowsSystemImage), err
}

func (s *rkeK8sWindowsSystemImageClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *rkeK8sWindowsSystemImageClient) AddHandler(ctx context.Context, name string, sync RKEK8sWindowsSystemImageHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync RKEK8sWindowsSystemImageHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddLifecycle(ctx context.Context, name string, lifecycle RKEK8sWindowsSystemImageLifecycle) {
	sync := NewRKEK8sWindowsSystemImageLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle RKEK8sWindowsSystemImageLifecycle) {
	sync := NewRKEK8sWindowsSystemImageLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync RKEK8sWindowsSystemImageHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync RKEK8sWindowsSystemImageHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle RKEK8sWindowsSystemImageLifecycle) {
	sync := NewRKEK8sWindowsSystemImageLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *rkeK8sWindowsSystemImageClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle RKEK8sWindowsSystemImageLifecycle) {
	sync := NewRKEK8sWindowsSystemImageLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type RKEK8sWindowsSystemImageIndexer func(obj *RKEK8sWindowsSystemImage) ([]string, error)

type RKEK8sWindowsSystemImageClientCache interface {
	Get(namespace, name string) (*RKEK8sWindowsSystemImage, error)
	List(namespace string, selector labels.Selector) ([]*RKEK8sWindowsSystemImage, error)

	Index(name string, indexer RKEK8sWindowsSystemImageIndexer)
	GetIndexed(name, key string) ([]*RKEK8sWindowsSystemImage, error)
}

type RKEK8sWindowsSystemImageClient interface {
	Create(*RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error)
	Get(namespace, name string, opts metav1.GetOptions) (*RKEK8sWindowsSystemImage, error)
	Update(*RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*RKEK8sWindowsSystemImageList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() RKEK8sWindowsSystemImageClientCache

	OnCreate(ctx context.Context, name string, sync RKEK8sWindowsSystemImageChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync RKEK8sWindowsSystemImageChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync RKEK8sWindowsSystemImageChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() RKEK8sWindowsSystemImageInterface
}

type rkeK8sWindowsSystemImageClientCache struct {
	client *rkeK8sWindowsSystemImageClient2
}

type rkeK8sWindowsSystemImageClient2 struct {
	iface      RKEK8sWindowsSystemImageInterface
	controller RKEK8sWindowsSystemImageController
}

func (n *rkeK8sWindowsSystemImageClient2) Interface() RKEK8sWindowsSystemImageInterface {
	return n.iface
}

func (n *rkeK8sWindowsSystemImageClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *rkeK8sWindowsSystemImageClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *rkeK8sWindowsSystemImageClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *rkeK8sWindowsSystemImageClient2) Create(obj *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error) {
	return n.iface.Create(obj)
}

func (n *rkeK8sWindowsSystemImageClient2) Get(namespace, name string, opts metav1.GetOptions) (*RKEK8sWindowsSystemImage, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *rkeK8sWindowsSystemImageClient2) Update(obj *RKEK8sWindowsSystemImage) (*RKEK8sWindowsSystemImage, error) {
	return n.iface.Update(obj)
}

func (n *rkeK8sWindowsSystemImageClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *rkeK8sWindowsSystemImageClient2) List(namespace string, opts metav1.ListOptions) (*RKEK8sWindowsSystemImageList, error) {
	return n.iface.List(opts)
}

func (n *rkeK8sWindowsSystemImageClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *rkeK8sWindowsSystemImageClientCache) Get(namespace, name string) (*RKEK8sWindowsSystemImage, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *rkeK8sWindowsSystemImageClientCache) List(namespace string, selector labels.Selector) ([]*RKEK8sWindowsSystemImage, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *rkeK8sWindowsSystemImageClient2) Cache() RKEK8sWindowsSystemImageClientCache {
	n.loadController()
	return &rkeK8sWindowsSystemImageClientCache{
		client: n,
	}
}

func (n *rkeK8sWindowsSystemImageClient2) OnCreate(ctx context.Context, name string, sync RKEK8sWindowsSystemImageChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &rkeK8sWindowsSystemImageLifecycleDelegate{create: sync})
}

func (n *rkeK8sWindowsSystemImageClient2) OnChange(ctx context.Context, name string, sync RKEK8sWindowsSystemImageChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &rkeK8sWindowsSystemImageLifecycleDelegate{update: sync})
}

func (n *rkeK8sWindowsSystemImageClient2) OnRemove(ctx context.Context, name string, sync RKEK8sWindowsSystemImageChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &rkeK8sWindowsSystemImageLifecycleDelegate{remove: sync})
}

func (n *rkeK8sWindowsSystemImageClientCache) Index(name string, indexer RKEK8sWindowsSystemImageIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*RKEK8sWindowsSystemImage); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *rkeK8sWindowsSystemImageClientCache) GetIndexed(name, key string) ([]*RKEK8sWindowsSystemImage, error) {
	var result []*RKEK8sWindowsSystemImage
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*RKEK8sWindowsSystemImage); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *rkeK8sWindowsSystemImageClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type rkeK8sWindowsSystemImageLifecycleDelegate struct {
	create RKEK8sWindowsSystemImageChangeHandlerFunc
	update RKEK8sWindowsSystemImageChangeHandlerFunc
	remove RKEK8sWindowsSystemImageChangeHandlerFunc
}

func (n *rkeK8sWindowsSystemImageLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *rkeK8sWindowsSystemImageLifecycleDelegate) Create(obj *RKEK8sWindowsSystemImage) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *rkeK8sWindowsSystemImageLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *rkeK8sWindowsSystemImageLifecycleDelegate) Remove(obj *RKEK8sWindowsSystemImage) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *rkeK8sWindowsSystemImageLifecycleDelegate) Updated(obj *RKEK8sWindowsSystemImage) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
