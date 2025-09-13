package wrangler

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	"github.com/sirupsen/logrus"
)

// DeferredInitializer describes how a specific client context (T) will be initialized during startup.
type DeferredInitializer[T any] interface {
	// WaitForClient is used to wait for a condition that must be satisfied before the initialization of
	// the deferred client context. This is typically an event handler watching a k8s resource.
	WaitForClient(context.Context) (T, error)
}

// DeferredRegistration provides a generic way to defer the execution of functions or registration of
// event handlers within the primary wrangler context. 'I' represents an implementation of DeferredInitializer,
// and is used to identify when the deferred functions can be executed. 'T' represents a scoped client
// context which will be passed to all deferred functions. This context is expected to hold the
// relevant clients, factories, and other resources which can only be accessed after initialization of 'I' is complete.
type DeferredRegistration[T any, I DeferredInitializer[T]] struct {
	Name string

	clientContext     T
	clientInitializer I

	clients           *Context
	registrationFuncs chan func(ctx context.Context, clients T) error
	funcs             chan func(clients T)
}

func NewDeferredRegistration[T any, I DeferredInitializer[T]](clients *Context, init I, name string) *DeferredRegistration[T, I] {
	return &DeferredRegistration[T, I]{
		Name:              name,
		clientInitializer: init,
		clients:           clients,
		registrationFuncs: make(chan func(ctx context.Context, clients T) error, 100),
		funcs:             make(chan func(clients T), 100),
	}
}

func (d *DeferredRegistration[T, I]) Manage(ctx context.Context) {
	go func() {
		var err error
		d.clientContext, err = d.clientInitializer.WaitForClient(ctx)
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
			logrus.Debugf("[%s - DeferRegistration] Executing deferred registration: %s", d.Name, getFuncName(f))
			if err := d.clients.StartFactoryWithTransaction(ctx, func(ctx context.Context) error {
				if err := f(ctx, d.clientContext); err != nil {
					return fmt.Errorf("failed to invoke registration func: %w", err)
				}
				return nil
			}); err != nil {
				return err
			}
		case f := <-d.funcs:
			logrus.Debugf("[%s - DeferFunc] Executing deferred function: %s", d.Name, getFuncName(f))
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
	logrus.Debugf("[%s - DeferFunc] Adding function to pool: %s", d.Name, getFuncName(f))
	d.funcs <- f
}

// DeferFuncWithError creates a new go routine which invokes f once the DeferredInitializer creates the client context.
// It returns an error channel to indicate if f encountered any errors during execution.
func (d *DeferredRegistration[T, I]) DeferFuncWithError(f func(wrangler T) error) chan error {
	logrus.Debugf("[%s - DeferFuncWithError] Adding function to pool: %s", d.Name, getFuncName(f))
	errChan := make(chan error, 1)
	d.DeferFunc(func(clients T) {
		defer close(errChan)

		if err := f(clients); err != nil {
			errChan <- err
		}
	})
	return errChan
}

// DeferRegistration enqueues a function to be executed once the client context has been initialized by adding it to the registration function pool.
// The functions passed to DeferRegistration are expected to register one or more event handlers which rely on deferred clients.
// Functions which must be deferred, but do not register event handlers, should be passed to DeferFunc instead.
// Calls to DeferRegistration are processed in the order they are made. Calls to DeferRegistration made after the client context has
// initialized will execute immediately, and the controller factory will be immediately started.
func (d *DeferredRegistration[T, I]) DeferRegistration(register func(ctx context.Context, clients T) error) {
	logrus.Debugf("[%s - DeferRegistration] Adding registration function to pool: %s", d.Name, getFuncName(register))
	d.registrationFuncs <- register
}

// getFuncName takes a function pointer (i.e. function name), and returns the function's name as a string.
// Go reflection is used to determine the value.
func getFuncName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
