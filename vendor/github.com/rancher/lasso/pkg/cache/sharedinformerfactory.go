package cache

import (
	"context"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/client"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type TweakListOptionsFunc func(*v1.ListOptions)

type SharedCacheFactoryOptions struct {
	DefaultResync    time.Duration
	DefaultNamespace string
	DefaultTweakList TweakListOptionsFunc

	KindResync    map[schema.GroupVersionKind]time.Duration
	KindNamespace map[schema.GroupVersionKind]string
	KindTweakList map[schema.GroupVersionKind]TweakListOptionsFunc
}

type sharedCacheFactory struct {
	lock sync.Mutex

	tweakList           TweakListOptionsFunc
	defaultResync       time.Duration
	defaultNamespace    string
	customResync        map[schema.GroupVersionKind]time.Duration
	customNamespaces    map[schema.GroupVersionKind]string
	customTweakList     map[schema.GroupVersionKind]TweakListOptionsFunc
	sharedClientFactory client.SharedClientFactory

	caches        map[schema.GroupVersionKind]cache.SharedIndexInformer
	startedCaches map[schema.GroupVersionKind]bool
}

// NewSharedInformerFactoryWithOptions constructs a new instance of a SharedInformerFactory with additional options.
func NewSharedCachedFactory(sharedClientFactory client.SharedClientFactory, opts *SharedCacheFactoryOptions) SharedCacheFactory {
	opts = applyDefaults(opts)

	factory := &sharedCacheFactory{
		lock:                sync.Mutex{},
		tweakList:           opts.DefaultTweakList,
		defaultResync:       opts.DefaultResync,
		defaultNamespace:    opts.DefaultNamespace,
		customResync:        opts.KindResync,
		customNamespaces:    opts.KindNamespace,
		customTweakList:     opts.KindTweakList,
		caches:              map[schema.GroupVersionKind]cache.SharedIndexInformer{},
		startedCaches:       map[schema.GroupVersionKind]bool{},
		sharedClientFactory: sharedClientFactory,
	}

	return factory
}

func applyDefaults(opts *SharedCacheFactoryOptions) *SharedCacheFactoryOptions {
	var newOpts SharedCacheFactoryOptions
	if opts != nil {
		newOpts = *opts
	}

	return &newOpts
}

func (f *sharedCacheFactory) Start(ctx context.Context) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.caches {
		if !f.startedCaches[informerType] {
			go informer.Run(ctx.Done())
			f.startedCaches[informerType] = true
		}
	}
}

func (f *sharedCacheFactory) WaitForCacheSync(ctx context.Context) map[schema.GroupVersionKind]bool {
	informers := func() map[schema.GroupVersionKind]cache.SharedIndexInformer {
		f.lock.Lock()
		defer f.lock.Unlock()

		informers := map[schema.GroupVersionKind]cache.SharedIndexInformer{}
		for informerType, informer := range f.caches {
			if f.startedCaches[informerType] {
				informers[informerType] = informer
			}
		}
		return informers
	}()

	res := map[schema.GroupVersionKind]bool{}
	for informType, informer := range informers {
		res[informType] = cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	}
	return res
}

func (f *sharedCacheFactory) ForObject(obj runtime.Object) (cache.SharedIndexInformer, error) {
	return f.ForKind(obj.GetObjectKind().GroupVersionKind())
}

func (f *sharedCacheFactory) ForKind(gvk schema.GroupVersionKind) (cache.SharedIndexInformer, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	informer, ok := f.caches[gvk]
	if ok {
		return informer, nil
	}

	resyncPeriod, ok := f.customResync[gvk]
	if !ok {
		resyncPeriod = f.defaultResync
	}

	namespace, ok := f.customNamespaces[gvk]
	if !ok {
		namespace = f.defaultNamespace
	}

	tweakList, ok := f.customTweakList[gvk]
	if !ok {
		tweakList = f.tweakList
	}

	obj, objList, err := f.sharedClientFactory.NewObjects(gvk)
	if err != nil {
		return nil, err
	}

	client, err := f.sharedClientFactory.ForKind(gvk)
	if err != nil {
		return nil, err
	}

	cache := NewCache(obj, objList, client, &Options{
		Namespace: namespace,
		Resync:    resyncPeriod,
		TweakList: tweakList,
	})
	f.caches[gvk] = cache

	return cache, nil
}

func (f *sharedCacheFactory) SharedClientFactory() client.SharedClientFactory {
	return f.sharedClientFactory
}

type SharedCacheFactory interface {
	Start(ctx context.Context)
	ForObject(obj runtime.Object) (cache.SharedIndexInformer, error)
	ForKind(gvk schema.GroupVersionKind) (cache.SharedIndexInformer, error)
	WaitForCacheSync(ctx context.Context) map[schema.GroupVersionKind]bool
	SharedClientFactory() client.SharedClientFactory
}
