package wrangler

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// DeferredInitializer describes how a specific client context (T) will be initialized during startup.
type DeferredInitializer[T any] interface {
	// OnChange is used to monitor a condition that must be satisfied before the initialization of
	// the deferred client context. This is typically an event handler watching a k8s resource.
	OnChange(context.Context, *Context)

	// GetClientContext returns the client context (T) after it has been initialized.
	GetClientContext(ctx context.Context) (T, error)
}

func NewBaseInitializer[T any]() *BaseInitializer[T] {
	return &BaseInitializer[T]{
		clientCreated: make(chan struct{}),
	}
}

// BaseInitializer sets and gets client contexts. This is a utility struct that should
// be embedded within more specific initializers which implement OnChange.
type BaseInitializer[T any] struct {
	clientContext T
	once          sync.Once
	clientCreated chan struct{}
}

func (b *BaseInitializer[T]) SetClientContext(clientContext T) {
	b.once.Do(func() {
		b.clientContext = clientContext
		close(b.clientCreated)
	})
}

func (b *BaseInitializer[T]) GetClientContext(ctx context.Context) (T, error) {
	select {
	case <-b.clientCreated:
		return b.clientContext, nil
	case <-ctx.Done():
		return b.clientContext, ctx.Err()
	}
}

// DeferredRegistration provides a generic way to defer the execution of functions or registration of
// event handlers within the primary wrangler context. I represents an implementation of DeferredInitializer,
// and is used to identify when the deferred functions can be executed. T represents a scoped client
// context which will be passed to all deferred functions. This context is expected to hold the
// relevant clients, factories, and other resources which can only be accessed after initialization of I is complete.
type DeferredRegistration[T any, I DeferredInitializer[T]] struct {
	clientContext     T
	clientInitializer I

	clients           *Context
	registrationFuncs chan func(ctx context.Context, clients T) error
	funcs             chan func(clients T)
}

func NewDeferredRegistration[T any, I DeferredInitializer[T]](clients *Context, init I) *DeferredRegistration[T, I] {
	return &DeferredRegistration[T, I]{
		clientInitializer: init,
		clients:           clients,
		registrationFuncs: make(chan func(ctx context.Context, clients T) error, 100),
		funcs:             make(chan func(clients T), 100),
	}
}

func (d *DeferredRegistration[T, I]) Manage(ctx context.Context) {
	go func() {
		d.clientInitializer.OnChange(ctx, d.clients)

		var err error
		d.clientContext, err = d.clientInitializer.GetClientContext(ctx)
		if err != nil {
			logrus.Fatalf("failed to get client context while managing deferred registration: %v", err)
		}

		if err = d.run(ctx); err != nil {
			logrus.Fatalf("failed to manage deferred registration: %v", err)
		}
	}()
}

func (d *DeferredRegistration[T, I]) run(ctx context.Context) error {
	for {
		select {
		case f := <-d.registrationFuncs:
			if err := d.clients.StartFactoryWithTransaction(ctx, func(ctx context.Context) error {
				if err := f(ctx, d.clientContext); err != nil {
					return fmt.Errorf("failed to invoke registration func: %w", err)
				}
				return nil
			}); err != nil {
				return err
			}
		case f := <-d.funcs:
			f(d.clientContext)
		case <-ctx.Done():
			return nil
		}
	}
}

// DeferFunc enqueues a function to be executed once the client context has been initialized by adding it to the function pool.
// Calls to DeferFunc are processed in the order they are made. Calls to DeferFunc made after the client context has initialized
// will execute immediately.
func (d *DeferredRegistration[T, I]) DeferFunc(f func(clients T)) {
	logrus.Debug("[DeferFunc] Adding function to pool")
	d.funcs <- f
}

// DeferFuncWithError creates a new go routine which invokes f once the DeferredInitializer creates the client context.
// It returns an error channel to indicate if f encountered any errors during execution.
func (d *DeferredRegistration[T, I]) DeferFuncWithError(f func(wrangler T) error) chan error {
	errChan := make(chan error, 1)
	d.funcs <- func(clients T) {
		defer close(errChan)

		if err := f(clients); err != nil {
			errChan <- err
		}
	}
	return errChan
}

// DeferRegistration enqueues a function to be executed once the client context has been initialized by adding it to the registration function pool.
// The functions passed to DeferRegistration are expected to register one or more event handlers which rely on deferred clients.
// Functions which must be deferred, but do not register event handlers, should be passed to DeferFunc instead.
// Calls to DeferRegistration are processed in the order they are made. Calls to DeferRegistration made after the client context has
// initialized will execute immediately, and the controller factory will be immediately started.
func (d *DeferredRegistration[T, I]) DeferRegistration(register func(ctx context.Context, clients T) error) {
	logrus.Debug("[DeferRegistration] Adding registration function to pool")
	d.registrationFuncs <- register
}
