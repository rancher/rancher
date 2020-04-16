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
	controllers        map[schema.GroupVersionKind]*sharedController

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
		controllers:        map[schema.GroupVersionKind]*sharedController{},
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
	for gvk, controller := range s.controllers {
		if err := controller.Start(ctx, s.getWorkers(gvk, defaultWorkers)); err != nil {
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
	gvk, err := s.sharedCacheFactory.SharedClientFactory().GVK(obj)
	if err != nil {
		return nil, err
	}
	return s.ForKind(gvk)
}

func (s *sharedControllerFactory) ForKind(gvk schema.GroupVersionKind) (SharedController, error) {
	controllerResult := s.getKind(gvk)
	if controllerResult != nil {
		return controllerResult, nil
	}

	s.controllerLock.Lock()
	defer s.controllerLock.Unlock()

	controllerResult = s.controllers[gvk]
	if controllerResult != nil {
		return controllerResult, nil
	}

	client, err := s.sharedCacheFactory.SharedClientFactory().ForKind(gvk)
	if err != nil {
		return nil, err
	}

	rateLimiter, ok := s.kindRateLimiter[gvk]
	if !ok {
		rateLimiter = s.rateLimiter
	}

	handler := &sharedHandler{}

	controllerResult = &sharedController{
		deferredController: func() (Controller, error) {
			cache, err := s.sharedCacheFactory.ForKind(gvk)
			if err != nil {
				return nil, err
			}

			c := New(gvk.String(), cache, handler, &Options{
				RateLimiter: rateLimiter,
			})
			return c, nil
		},
		handler: handler,
		client:  client,
	}

	if s.started {
		if err := controllerResult.Start(s.runningContext, s.getWorkers(gvk, 0)); err != nil {
			return nil, err
		}
	}

	s.controllers[gvk] = controllerResult
	return controllerResult, nil
}

func (s *sharedControllerFactory) getWorkers(gvk schema.GroupVersionKind, workers int) int {
	w, ok := s.kindWorkers[gvk]
	if ok {
		return w
	}
	if workers > 0 {
		return workers
	}
	return s.workers
}

func (s *sharedControllerFactory) getKind(gvk schema.GroupVersionKind) *sharedController {
	s.controllerLock.RLock()
	defer s.controllerLock.RUnlock()
	return s.controllers[gvk]
}

func (s *sharedControllerFactory) SharedCacheFactory() cache.SharedCacheFactory {
	return s.sharedCacheFactory
}
