package wrangler

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

// DeferredRegistration is the manager structure used to keep deferred
// registrations and functions for a set of requirements. The deferred elements
// are stored in channels, with go managing access, and avoiding mutexes
type DeferredRegistration struct {
	mutex             sync.Mutex                                            // Serialize access to this structure
	client            *Context                                              // Context to update and operate in
	registrationFuncs chan func(ctx context.Context, client *Context) error // Deferred registrations
	funcs             chan func(client *Context)                            // Deferred functions
}

// newDeferredRegistration creates a deferral manager for the given wrangler
// context.
func newDeferredRegistration(client *Context) *DeferredRegistration {
	return &DeferredRegistration{
		client:            client,
		registrationFuncs: make(chan func(ctx context.Context, client *Context) error, 100),
		funcs:             make(chan func(client *Context), 100),
	}
}

// Run executes all the deferred registrations and functions. It is called from
// the goroutine waiting for the requirements of this particular deferral
// manager, when all requirements are available. It never returns, the calling
// goroutine never stops.
func (d *DeferredRegistration) Run(ctx context.Context) error {
	for {
		select {
		case f := <-d.registrationFuncs:
			if err := d.client.StartFactoryWithTransaction(ctx, func(ctx context.Context) error {
				logrus.Debugf("[deferred-registration] %p run  registration %q", d.client, fn(f))
				if err := f(ctx, d.client); err != nil {
					logrus.Debugf("[deferred-registration] %p fail registration %q, error: %v", d.client, fn(f), err)
					return fmt.Errorf("failed to invoke reg func: %w", err)
				}
				logrus.Debugf("[deferred-registration] %p done registration %q", d.client, fn(f))
				return nil
			}); err != nil {
				return err
			}
		case f := <-d.funcs:
			logrus.Debugf("[deferred-registration] %p run  func %q", d.client, fn(f))
			f(d.client)
			logrus.Debugf("[deferred-registration] %p done func %q", d.client, fn(f))
		case <-ctx.Done():
			return nil
		}
	}
}

// DeferFunc enqueues a function to be executed once the requirements of the
// manager are available, by adding it to the function pool.  Calls to DeferFunc
// are processed in the order they are made. Calls to DeferFunc made after the
// requirements are available will execute near immediately.
func (d *DeferredRegistration) DeferFunc(f func(client *Context)) {
	logrus.Debugf("[deferred-registration] DeferFunc adding function %q", fn(f))
	d.funcs <- f
}

// DeferFuncWithError is like DeferFunc, except that it (a) accepts a function
// with an error return, and (b) returns an error channel from which the
// function's error state can be read from after it was run.
func (d *DeferredRegistration) DeferFuncWithError(f func(wrangler *Context) error) chan error {
	errChan := make(chan error, 1)
	logrus.Debugf("[deferred-registration] DeferFuncWithError adding function %q", fn(f))
	d.DeferFunc(func(client *Context) {
		defer close(errChan)

		if err := f(client); err != nil {
			errChan <- err
		}
	})
	return errChan
}

// DeferRegistration enqueues a function to be executed once the requirements
// are available, by adding it to the registration function pool.  The functions
// passed to DeferRegistration are expected to register one or more event
// handlers.  Functions which must be deferred, but do not register event
// handlers, should be passed to DeferFunc instead.  Calls to DeferRegistration
// are processed in the order they are made. Calls to DeferRegistration made
// after the requirements are available will execute immediately, and the
// controller factory will be immediately started.
func (d *DeferredRegistration) DeferRegistration(register func(ctx context.Context, clients *Context) error) {
	logrus.Debugf("[deferred-registration] DeferRegistration adding function %q", fn(register))
	d.registrationFuncs <- register
}

func fn(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
