package wrangler

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const pollInterval = 5 * time.Second

// ManageDeferrals polls for the availability of the requirements implied in the
// `poller` function. On success it stops polling, performs the `setup` and then
// executes all registrations and functions found in the registration manager
// `d`. All registrations and functions added to the same `d` after polling is
// done will execute immediately.
func (w *Context) ManageDeferrals(ctx context.Context,
	label string,
	d *DeferredRegistration,
	poller func(w *Context) bool,
	setup func(w *Context),
) {
	// Prevent the start of pollers after polling was completed.
	d.mutex.Lock()
	if d.Initialized {
		d.mutex.Unlock()
		return
	}
	d.mutex.Unlock()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	logrus.Debugf("[deferred-registration] %p starting to monitor %s", w, label)

	// Wait until the reqirements are met, as per the `poller`
	for {
		if allIsReady := poller(w); allIsReady {
			logrus.Debugf("[deferred-registration] %p all requirements now available and established.", w)
			break
		}

		select {
		case <-ctx.Done():
			logrus.Error("[deferred-registration] Context cancelled while waiting for requirements")
			return
		case <-ticker.C:
		}
	}

	// Complete the setup after polling was sucessful, then handle the callbacks
	setup(w)

	w.initializeFactory(ctx, d)
}

// initializeFactory runs all registrations and functions added to the
// registration manager `d`. The exact context depends on if the relevant
// wrangler context is already active (started), or not. When started this is
// done in a transaction, else without. The function __will not__ start an
// inactive context. It will restart an active context to pick up on the new
// elements.
func (w *Context) initializeFactory(ctx context.Context, d *DeferredRegistration) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	logrus.Debugf("[deferred-registration] %p initialize factory", w)

	// If the larger wrangler context has not started yet, do not start it prematurely
	invoked := func() bool {
		w.controllerLock.Lock()
		defer w.controllerLock.Unlock()

		if wranglerStarted := w.started; wranglerStarted {
			return false
		}

		logrus.Debugf("[deferred-registration] %p run deferred registrations and funcs for inactive wrangler", w)
		if err := d.invokePools(ctx, w); err != nil {
			logrus.Fatalf("[deferred-registration] %p Encountered unexpected error while invoking deferred pools: %v", w, err)
		}

		logrus.Debugf("[deferred-registration] %p mark initialized", w)

		d.Initialized = true

		logrus.Debugf("[deferred-registration] %p initialize factory done, inactive wrangler, not started", w)
		return true
	}()
	if invoked {
		return
	}

	// As wrangler has already started, start the factory again to pick up new registrations
	if err := w.StartSharedFactoryWithTransaction(ctx, func(ctx context.Context) error {
		logrus.Debugf("[deferred-registration] %p run deferred registrations and funcs for active wrangler", w)
		if err := d.invokePools(ctx, w); err != nil {
			logrus.Fatalf("[deferred-registration] %p Encountered unexpected error while invoking deferred pools: %v", w, err)
		}

		logrus.Debugf("[deferred-registration] %p mark initialized", w)

		d.Initialized = true

		logrus.Debugf("[deferred-registration] %p initialize factory done, active wrangler", w)
		return nil
	}); err != nil {
		logrus.Fatalf("[deferred-registration] %p failed to invoke deferrred function pools", w)
	}

	logrus.Debugf("[deferred-registration] %p initialize factory, active wrangler, restarting factory", w)

	if err := w.ControllerFactory.Start(ctx, defaultControllerWorkerCount); err != nil {
		logrus.Fatalf("[deferred-registration] %p Encountered unexpected error while restarting factory: %v", w, err)
	}
}

// DeferredRegistration is the manager structure used to keep deferred
// registrations and functions for a set of requirements.
type DeferredRegistration struct {
	Initialized       bool                                                // Set when `ManageDeferrals` has completed.
	wg                *sync.WaitGroup                                     // Group (Number) of deferred calls, funcs and registrations.
	mutex             sync.Mutex                                          // Serialize access to this structure
	registrationFuncs []func(ctx context.Context, clients *Context) error // Deferred registrations
	funcs             []func(clients *Context)                            // Deferred funcs
}

// invokePools executes the registrations and functions held by registration
// manager `d`.  `d`'s lock has to be held for exclusion before calling this
// function. When the function returns the sets of registrations and functions
// to call are reset to empty.
func (d *DeferredRegistration) invokePools(ctx context.Context, clients *Context) error {
	if err := d.invokeRegistrationFuncs(ctx, clients, d.registrationFuncs); err != nil {
		return err
	}

	for _, f := range d.funcs {
		logrus.Debugf("[deferred-registration] %p run  func  %v", clients, fn(f))
		f(clients)
		logrus.Debugf("[deferred-registration] %p done func %v", clients, fn(f))
		d.wg.Done() // [1]
	}

	d.registrationFuncs = []func(ctx context.Context, clients *Context) error{}
	d.funcs = []func(clients *Context){}

	return nil
}

// DeferFunc registers a function to be invoked when `d`'s requirements are
// available and ready. All functions of this kind are executed in order of
// registration, and after all functions registered via `DeferRegistration`.
// BEWARE, if this function is invoked when `d` is already marked as ready, then
// the function is called immediately.
func (d *DeferredRegistration) DeferFunc(clients *Context, f func(clients *Context)) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if !d.Initialized {
		logrus.Debugf("[deferred-registration] %p defer func %v", clients, fn(f))
		d.wg.Add(1) // Released at [1]
		d.funcs = append(d.funcs, f)
		return
	}

	logrus.Debugf("[deferred-registration] %p imm run func  %v", clients, fn(f))
	f(clients)
	logrus.Debugf("[deferred-registration] %p imm done func %v", clients, fn(f))
}

// DeferFuncWithError registers a function to be invoked when `d` requirements
// are available and ready. It returns an error channel where callers can read
// the error status of the function after its execution.  All functions of this
// kind are executed after all functions registered via `DeferFunc` or
// `DeferRegistration`.  The order of execution within the group is
// undetermined.
// BEWARE, if this function is invoked when `d` is already marked as ready, then
// the function is called immediately.
func (d *DeferredRegistration) DeferFuncWithError(clients *Context, f func(wrangler *Context) error) chan error {
	errChan := make(chan error, 1)
	go func(errs chan error) {
		d.wg.Wait()
		err := f(clients)
		defer close(errChan)

		if err != nil {
			errChan <- err
		}
	}(errChan)
	return errChan
}

// DeferRegistration registers a function to be invoked when `d`'s requirements
// are available and ready. All functions registered here are executed in order
// of registration, within a single transaction, and before all functions
// registered with `DeferFunc` or `DeferFuncWithError`.
// BEWARE, if this function is invoked when `d` is already marked as ready, then
// the function is called immediately.
func (d *DeferredRegistration) DeferRegistration(ctx context.Context, clients *Context,
	register func(ctx context.Context, clients *Context) error) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.wg.Add(1) // Released at [2], inside `invokeRegistrationFuncs`, now or deferred

	if !d.Initialized {
		logrus.Debugf("[deferred-registration] %p defer registration %v", clients, fn(register))
		d.registrationFuncs = append(d.registrationFuncs, register)
		return nil
	}

	logrus.Debugf("[deferred-registration] %p immediate registration %v", clients, fn(register))

	invoked, err := func() (bool, error) {
		clients.controllerLock.Lock()
		defer clients.controllerLock.Unlock()

		if wranglerStarted := clients.started; wranglerStarted {
			return false, nil
		}

		logrus.Debugf("[deferred-registration] %p DeferRegistration, inactive wrangler, skip factory start", clients)
		defer func() {
			logrus.Debugf("[deferred-registration] %p DeferRegistration, inactive wrangler, skip factory start, done", clients)
		}()
		return true, d.invokeRegistrationFuncs(ctx, clients,
			[]func(ctx context.Context, clients *Context) error{
				register,
			})
	}()
	if err != nil {
		return err
	}
	if invoked {
		return nil
	}

	logrus.Debugf("[deferred-registration] %p DeferRegistration, active wrangler, restarting factory", clients)
	defer func() {
		logrus.Debugf("[deferred-registration] %p DeferRegistration, active wrangler context, restarting factory done", clients)
	}()

	return clients.StartSharedFactoryWithTransaction(ctx, func(ctx context.Context) error {
		return d.invokeRegistrationFuncs(ctx, clients,
			[]func(ctx context.Context, clients *Context) error{
				register,
			})
	})
}

func (d *DeferredRegistration) invokeRegistrationFuncs(transaction context.Context, clients *Context,
	f []func(ctx context.Context, clients *Context) error) error {
	for _, register := range f {
		logrus.Debugf("[deferred-registration] %p run registration  %v", clients, fn(register))
		if err := register(transaction, clients); err != nil {
			logrus.Debugf("[deferred-registration] %p fail registration %v, error: %v", clients, fn(register), err)
			return err
		}
		logrus.Debugf("[deferred-registration] %p done registration %v", clients, fn(register))
		d.wg.Done() // [2]
	}
	return nil
}

func fn(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
