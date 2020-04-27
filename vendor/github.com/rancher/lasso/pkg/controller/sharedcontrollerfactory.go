package controller

import (
	"context"
	"sync"

	"github.com/rancher/lasso/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"github.com/rancher/lasso/pkg/cache"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
)

type SharedControllerFactory interface {
	ForObject(obj runtime.Object) (SharedController, error)
	ForKind(gvk schema.GroupVersionKind) (SharedController, error)
	ForResource(gvr schema.GroupVersionResource, namespaced bool) SharedController
	SharedCacheFactory() cache.SharedCacheFactory
	Start(ctx context.Context, workers int) error
}

type SharedControllerFactoryOptions struct {
	DefaultRateLimiter workqueue.RateLimiter
	DefaultWorkers     int

	KindRateLimiter map[schema.GroupVersionKind]workqueue.RateLimiter
	KindWorkers     map[schema.GroupVersionKind]int
}

type sharedControllerFactory struct {
	controllerLock sync.RWMutex

	sharedCacheFactory cache.SharedCacheFactory
	controllers        map[schema.GroupVersionResource]*sharedController

	started        bool
	runningContext context.Context

	rateLimiter     workqueue.RateLimiter
	workers         int
	kindRateLimiter map[schema.GroupVersionKind]workqueue.RateLimiter
	kindWorkers     map[schema.GroupVersionKind]int
}

func NewSharedControllerFactoryFromConfig(config *rest.Config, scheme *runtime.Scheme) (SharedControllerFactory, error) {
	cf, err := client.NewSharedClientFactory(config, &client.SharedClientFactoryOptions{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	return NewSharedControllerFactory(cache.NewSharedCachedFactory(cf, nil), nil), nil
}

func NewSharedControllerFactory(cacheFactory cache.SharedCacheFactory, opts *SharedControllerFactoryOptions) SharedControllerFactory {
	opts = applyDefaultSharedOptions(opts)
	return &sharedControllerFactory{
		sharedCacheFactory: cacheFactory,
		controllers:        map[schema.GroupVersionResource]*sharedController{},
		workers:            opts.DefaultWorkers,
		kindWorkers:        opts.KindWorkers,
		rateLimiter:        opts.DefaultRateLimiter,
		kindRateLimiter:    opts.KindRateLimiter,
	}
}

func applyDefaultSharedOptions(opts *SharedControllerFactoryOptions) *SharedControllerFactoryOptions {
	var newOpts SharedControllerFactoryOptions
	if opts != nil {
		newOpts = *opts
	}
	if newOpts.DefaultWorkers == 0 {
		newOpts.DefaultWorkers = 5
	}
	return &newOpts
}

func (s *sharedControllerFactory) Start(ctx context.Context, defaultWorkers int) error {
	s.controllerLock.Lock()
	defer s.controllerLock.Unlock()

	s.sharedCacheFactory.Start(ctx)
	for gvr, controller := range s.controllers {
		w, err := s.getWorkers(gvr, defaultWorkers)
		if err != nil {
			return err
		}
		if err := controller.Start(ctx, w); err != nil {
			return err
		}
	}

	s.started = true
	s.runningContext = ctx

	go func() {
		<-ctx.Done()

		s.controllerLock.Lock()
		defer s.controllerLock.Unlock()

		s.started = false
		s.runningContext = nil
	}()

	return nil
}

func (s *sharedControllerFactory) ForObject(obj runtime.Object) (SharedController, error) {
	gvk, err := s.sharedCacheFactory.SharedClientFactory().GVKForObject(obj)
	if err != nil {
		return nil, err
	}
	return s.ForKind(gvk)
}

func (s *sharedControllerFactory) ForKind(gvk schema.GroupVersionKind) (SharedController, error) {
	gvr, nsed, err := s.sharedCacheFactory.SharedClientFactory().ResourceForGVK(gvk)
	if err != nil {
		return nil, err
	}

	return s.ForResource(gvr, nsed), nil
}

func (s *sharedControllerFactory) ForResource(gvr schema.GroupVersionResource, namespaced bool) SharedController {
	controllerResult := s.byResource(gvr)
	if controllerResult != nil {
		return controllerResult
	}

	s.controllerLock.Lock()
	defer s.controllerLock.Unlock()

	controllerResult = s.controllers[gvr]
	if controllerResult != nil {
		return controllerResult
	}

	client := s.sharedCacheFactory.SharedClientFactory().ForResource(gvr, namespaced)

	handler := &sharedHandler{}

	controllerResult = &sharedController{
		deferredController: func() (Controller, error) {
			gvk, err := s.sharedCacheFactory.SharedClientFactory().GVKForResource(gvr)
			if err != nil {
				return nil, err
			}

			cache, err := s.sharedCacheFactory.ForKind(gvk)
			if err != nil {
				return nil, err
			}

			rateLimiter, ok := s.kindRateLimiter[gvk]
			if !ok {
				rateLimiter = s.rateLimiter
			}

			c := New(gvk.String(), cache, handler, &Options{
				RateLimiter: rateLimiter,
			})

			var workers int
			if s.started {
				workers, err = s.getWorkers(gvr, 0)
				go controllerResult.Start(s.runningContext, workers)
			}

			return c, err
		},
		handler: handler,
		client:  client,
	}

	s.controllers[gvr] = controllerResult
	return controllerResult
}

func (s *sharedControllerFactory) getWorkers(gvr schema.GroupVersionResource, workers int) (int, error) {
	gvk, err := s.sharedCacheFactory.SharedClientFactory().GVKForResource(gvr)
	if err != nil {
		return 0, err
	}

	w, ok := s.kindWorkers[gvk]
	if ok {
		return w, nil
	}
	if workers > 0 {
		return workers, nil
	}
	return s.workers, nil
}

func (s *sharedControllerFactory) byResource(gvr schema.GroupVersionResource) *sharedController {
	s.controllerLock.RLock()
	defer s.controllerLock.RUnlock()
	return s.controllers[gvr]
}

func (s *sharedControllerFactory) SharedCacheFactory() cache.SharedCacheFactory {
	return s.sharedCacheFactory
}
