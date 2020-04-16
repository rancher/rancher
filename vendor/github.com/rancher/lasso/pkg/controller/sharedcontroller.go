package controller

import (
	"context"
	"sync"
	"time"

	cachetools "k8s.io/client-go/tools/cache"

	"github.com/rancher/lasso/pkg/cache"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/rancher/lasso/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
)

type SharedControllerHandler interface {
	OnChange(key string, obj runtime.Object) (runtime.Object, error)
}

type SharedController interface {
	Controller

	RegisterHandler(ctx context.Context, name string, handler SharedControllerHandler)
	Client() *client.Client
}

type SharedControllerHandlerFunc func(key string, obj runtime.Object) (runtime.Object, error)

func (s SharedControllerHandlerFunc) OnChange(key string, obj runtime.Object) (runtime.Object, error) {
	return s(key, obj)
}

type sharedController struct {
	// this allows one to create a sharedcontroller but it will not actually be started
	// unless some aspect of the controllers informer is accessed or needed to be used
	deferredController func() (Controller, error)
	sharedCacheFactory cache.SharedCacheFactory
	controller         Controller
	gvk                schema.GroupVersionKind
	handler            *sharedHandler
	startLock          sync.Mutex
	started            bool
	startError         error
	client             *client.Client
}

func (s *sharedController) Enqueue(namespace, name string) {
	s.initController().Enqueue(namespace, name)
}

func (s *sharedController) EnqueueAfter(namespace, name string, delay time.Duration) {
	s.initController().EnqueueAfter(namespace, name, delay)
}

func (s *sharedController) EnqueueKey(key string) {
	s.initController().EnqueueKey(key)
}

func (s *sharedController) Informer() cachetools.SharedIndexInformer {
	return s.initController().Informer()
}

func (s *sharedController) Client() *client.Client {
	return s.client
}

func (s *sharedController) initController() Controller {
	s.startLock.Lock()
	defer s.startLock.Unlock()

	if s.controller != nil {
		return s.controller
	}

	controller, err := s.deferredController()
	if err != nil {
		controller = newErrorController()
	}

	s.startError = err
	s.controller = controller
	return s.controller
}

func (s *sharedController) Start(ctx context.Context, workers int) error {
	s.startLock.Lock()
	defer s.startLock.Unlock()

	if s.startError != nil || s.controller == nil {
		return s.startError
	}

	if err := s.controller.Start(ctx, workers); err != nil {
		return err
	}
	s.started = true

	go func() {
		<-ctx.Done()
		s.startLock.Lock()
		defer s.startLock.Unlock()
		s.started = false
	}()

	return nil
}

func (s *sharedController) RegisterHandler(ctx context.Context, name string, handler SharedControllerHandler) {
	getHandlerTransaction(ctx).do(func() {
		s.startLock.Lock()
		defer s.startLock.Unlock()

		s.handler.Register(ctx, name, handler)
		if s.started {
			c := s.initController()
			for _, key := range c.Informer().GetStore().ListKeys() {
				c.EnqueueKey(key)
			}
		}
	})
}
