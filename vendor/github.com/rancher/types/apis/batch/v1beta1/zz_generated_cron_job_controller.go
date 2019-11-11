package v1beta1

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/resource"
	"k8s.io/api/batch/v1beta1"
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
	CronJobGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "CronJob",
	}
	CronJobResource = metav1.APIResource{
		Name:         "cronjobs",
		SingularName: "cronjob",
		Namespaced:   true,

		Kind: CronJobGroupVersionKind.Kind,
	}

	CronJobGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  Version,
		Resource: "cronjobs",
	}
)

func init() {
	resource.Put(CronJobGroupVersionResource)
}

func NewCronJob(namespace, name string, obj v1beta1.CronJob) *v1beta1.CronJob {
	obj.APIVersion, obj.Kind = CronJobGroupVersionKind.ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

type CronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []v1beta1.CronJob `json:"items"`
}

type CronJobHandlerFunc func(key string, obj *v1beta1.CronJob) (runtime.Object, error)

type CronJobChangeHandlerFunc func(obj *v1beta1.CronJob) (runtime.Object, error)

type CronJobLister interface {
	List(namespace string, selector labels.Selector) (ret []*v1beta1.CronJob, err error)
	Get(namespace, name string) (*v1beta1.CronJob, error)
}

type CronJobController interface {
	Generic() controller.GenericController
	Informer() cache.SharedIndexInformer
	Lister() CronJobLister
	AddHandler(ctx context.Context, name string, handler CronJobHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CronJobHandlerFunc)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, handler CronJobHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, handler CronJobHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type CronJobInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*v1beta1.CronJob) (*v1beta1.CronJob, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.CronJob, error)
	Get(name string, opts metav1.GetOptions) (*v1beta1.CronJob, error)
	Update(*v1beta1.CronJob) (*v1beta1.CronJob, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*CronJobList, error)
	ListNamespaced(namespace string, opts metav1.ListOptions) (*CronJobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() CronJobController
	AddHandler(ctx context.Context, name string, sync CronJobHandlerFunc)
	AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CronJobHandlerFunc)
	AddLifecycle(ctx context.Context, name string, lifecycle CronJobLifecycle)
	AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CronJobLifecycle)
	AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CronJobHandlerFunc)
	AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CronJobHandlerFunc)
	AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CronJobLifecycle)
	AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CronJobLifecycle)
}

type cronJobLister struct {
	controller *cronJobController
}

func (l *cronJobLister) List(namespace string, selector labels.Selector) (ret []*v1beta1.CronJob, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*v1beta1.CronJob))
	})
	return
}

func (l *cronJobLister) Get(namespace, name string) (*v1beta1.CronJob, error) {
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
			Group:    CronJobGroupVersionKind.Group,
			Resource: "cronJob",
		}, key)
	}
	return obj.(*v1beta1.CronJob), nil
}

type cronJobController struct {
	controller.GenericController
}

func (c *cronJobController) Generic() controller.GenericController {
	return c.GenericController
}

func (c *cronJobController) Lister() CronJobLister {
	return &cronJobLister{
		controller: c,
	}
}

func (c *cronJobController) AddHandler(ctx context.Context, name string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.CronJob); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cronJobController) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.CronJob); ok {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cronJobController) AddClusterScopedHandler(ctx context.Context, name, cluster string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.CronJob); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

func (c *cronJobController) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, cluster string, handler CronJobHandlerFunc) {
	c.GenericController.AddHandler(ctx, name, func(key string, obj interface{}) (interface{}, error) {
		if !enabled() {
			return nil, nil
		} else if obj == nil {
			return handler(key, nil)
		} else if v, ok := obj.(*v1beta1.CronJob); ok && controller.ObjectInCluster(cluster, obj) {
			return handler(key, v)
		} else {
			return nil, nil
		}
	})
}

type cronJobFactory struct {
}

func (c cronJobFactory) Object() runtime.Object {
	return &v1beta1.CronJob{}
}

func (c cronJobFactory) List() runtime.Object {
	return &CronJobList{}
}

func (s *cronJobClient) Controller() CronJobController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.cronJobControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(CronJobGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &cronJobController{
		GenericController: genericController,
	}

	s.client.cronJobControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type cronJobClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   CronJobController
}

func (s *cronJobClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *cronJobClient) Create(o *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) Get(name string, opts metav1.GetOptions) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) Update(o *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *cronJobClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *cronJobClient) List(opts metav1.ListOptions) (*CronJobList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*CronJobList), err
}

func (s *cronJobClient) ListNamespaced(namespace string, opts metav1.ListOptions) (*CronJobList, error) {
	obj, err := s.objectClient.ListNamespaced(namespace, opts)
	return obj.(*CronJobList), err
}

func (s *cronJobClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *cronJobClient) Patch(o *v1beta1.CronJob, patchType types.PatchType, data []byte, subresources ...string) (*v1beta1.CronJob, error) {
	obj, err := s.objectClient.Patch(o.Name, o, patchType, data, subresources...)
	return obj.(*v1beta1.CronJob), err
}

func (s *cronJobClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *cronJobClient) AddHandler(ctx context.Context, name string, sync CronJobHandlerFunc) {
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cronJobClient) AddFeatureHandler(ctx context.Context, enabled func() bool, name string, sync CronJobHandlerFunc) {
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cronJobClient) AddLifecycle(ctx context.Context, name string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddHandler(ctx, name, sync)
}

func (s *cronJobClient) AddFeatureLifecycle(ctx context.Context, enabled func() bool, name string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name, false, s, lifecycle)
	s.Controller().AddFeatureHandler(ctx, enabled, name, sync)
}

func (s *cronJobClient) AddClusterScopedHandler(ctx context.Context, name, clusterName string, sync CronJobHandlerFunc) {
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedFeatureHandler(ctx context.Context, enabled func() bool, name, clusterName string, sync CronJobHandlerFunc) {
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedLifecycle(ctx context.Context, name, clusterName string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedHandler(ctx, name, clusterName, sync)
}

func (s *cronJobClient) AddClusterScopedFeatureLifecycle(ctx context.Context, enabled func() bool, name, clusterName string, lifecycle CronJobLifecycle) {
	sync := NewCronJobLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.Controller().AddClusterScopedFeatureHandler(ctx, enabled, name, clusterName, sync)
}

type CronJobIndexer func(obj *v1beta1.CronJob) ([]string, error)

type CronJobClientCache interface {
	Get(namespace, name string) (*v1beta1.CronJob, error)
	List(namespace string, selector labels.Selector) ([]*v1beta1.CronJob, error)

	Index(name string, indexer CronJobIndexer)
	GetIndexed(name, key string) ([]*v1beta1.CronJob, error)
}

type CronJobClient interface {
	Create(*v1beta1.CronJob) (*v1beta1.CronJob, error)
	Get(namespace, name string, opts metav1.GetOptions) (*v1beta1.CronJob, error)
	Update(*v1beta1.CronJob) (*v1beta1.CronJob, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	List(namespace string, opts metav1.ListOptions) (*CronJobList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)

	Cache() CronJobClientCache

	OnCreate(ctx context.Context, name string, sync CronJobChangeHandlerFunc)
	OnChange(ctx context.Context, name string, sync CronJobChangeHandlerFunc)
	OnRemove(ctx context.Context, name string, sync CronJobChangeHandlerFunc)
	Enqueue(namespace, name string)

	Generic() controller.GenericController
	ObjectClient() *objectclient.ObjectClient
	Interface() CronJobInterface
}

type cronJobClientCache struct {
	client *cronJobClient2
}

type cronJobClient2 struct {
	iface      CronJobInterface
	controller CronJobController
}

func (n *cronJobClient2) Interface() CronJobInterface {
	return n.iface
}

func (n *cronJobClient2) Generic() controller.GenericController {
	return n.iface.Controller().Generic()
}

func (n *cronJobClient2) ObjectClient() *objectclient.ObjectClient {
	return n.Interface().ObjectClient()
}

func (n *cronJobClient2) Enqueue(namespace, name string) {
	n.iface.Controller().Enqueue(namespace, name)
}

func (n *cronJobClient2) Create(obj *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	return n.iface.Create(obj)
}

func (n *cronJobClient2) Get(namespace, name string, opts metav1.GetOptions) (*v1beta1.CronJob, error) {
	return n.iface.GetNamespaced(namespace, name, opts)
}

func (n *cronJobClient2) Update(obj *v1beta1.CronJob) (*v1beta1.CronJob, error) {
	return n.iface.Update(obj)
}

func (n *cronJobClient2) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	return n.iface.DeleteNamespaced(namespace, name, options)
}

func (n *cronJobClient2) List(namespace string, opts metav1.ListOptions) (*CronJobList, error) {
	return n.iface.List(opts)
}

func (n *cronJobClient2) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return n.iface.Watch(opts)
}

func (n *cronJobClientCache) Get(namespace, name string) (*v1beta1.CronJob, error) {
	return n.client.controller.Lister().Get(namespace, name)
}

func (n *cronJobClientCache) List(namespace string, selector labels.Selector) ([]*v1beta1.CronJob, error) {
	return n.client.controller.Lister().List(namespace, selector)
}

func (n *cronJobClient2) Cache() CronJobClientCache {
	n.loadController()
	return &cronJobClientCache{
		client: n,
	}
}

func (n *cronJobClient2) OnCreate(ctx context.Context, name string, sync CronJobChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-create", &cronJobLifecycleDelegate{create: sync})
}

func (n *cronJobClient2) OnChange(ctx context.Context, name string, sync CronJobChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name+"-change", &cronJobLifecycleDelegate{update: sync})
}

func (n *cronJobClient2) OnRemove(ctx context.Context, name string, sync CronJobChangeHandlerFunc) {
	n.loadController()
	n.iface.AddLifecycle(ctx, name, &cronJobLifecycleDelegate{remove: sync})
}

func (n *cronJobClientCache) Index(name string, indexer CronJobIndexer) {
	err := n.client.controller.Informer().GetIndexer().AddIndexers(map[string]cache.IndexFunc{
		name: func(obj interface{}) ([]string, error) {
			if v, ok := obj.(*v1beta1.CronJob); ok {
				return indexer(v)
			}
			return nil, nil
		},
	})

	if err != nil {
		panic(err)
	}
}

func (n *cronJobClientCache) GetIndexed(name, key string) ([]*v1beta1.CronJob, error) {
	var result []*v1beta1.CronJob
	objs, err := n.client.controller.Informer().GetIndexer().ByIndex(name, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if v, ok := obj.(*v1beta1.CronJob); ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (n *cronJobClient2) loadController() {
	if n.controller == nil {
		n.controller = n.iface.Controller()
	}
}

type cronJobLifecycleDelegate struct {
	create CronJobChangeHandlerFunc
	update CronJobChangeHandlerFunc
	remove CronJobChangeHandlerFunc
}

func (n *cronJobLifecycleDelegate) HasCreate() bool {
	return n.create != nil
}

func (n *cronJobLifecycleDelegate) Create(obj *v1beta1.CronJob) (runtime.Object, error) {
	if n.create == nil {
		return obj, nil
	}
	return n.create(obj)
}

func (n *cronJobLifecycleDelegate) HasFinalize() bool {
	return n.remove != nil
}

func (n *cronJobLifecycleDelegate) Remove(obj *v1beta1.CronJob) (runtime.Object, error) {
	if n.remove == nil {
		return obj, nil
	}
	return n.remove(obj)
}

func (n *cronJobLifecycleDelegate) Updated(obj *v1beta1.CronJob) (runtime.Object, error) {
	if n.update == nil {
		return obj, nil
	}
	return n.update(obj)
}
