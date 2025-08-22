package wrangler

import (
	"context"
	"sync"
	"time"

	capi "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManageDeferredCAPIContext polls for the availability of CAPI CRDs and registers deferred controllers
// and executes deferred functions once they are available. Once CAPI CRDs are found, this function will
// not continue polling. Individual registration calls can be made once polling is complete by directly using
// the DeferredCAPIRegistration struct.
func (w *Context) ManageDeferredCAPIContext(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logrus.Info("[deferred-capi - ManageDeferredCAPIContext] Starting to monitor CAPI CRD availability")

	for {
		allCRDsReady := w.checkCAPICRDs()
		if allCRDsReady {
			logrus.Debug("[deferred-capi - ManageDeferredCAPIContext] All CAPI CRDs are now available and established.")
			w.initializeCAPIFactory(ctx)
			return
		}

		select {
		case <-ctx.Done():
			logrus.Error("[deferred-capi - ManageDeferredCAPIContext] Context cancelled while waiting for CAPI CRDs")
			return
		case <-ticker.C:
		}
	}
}

func (w *Context) checkCAPICRDs() bool {
	requiredCRDs := []string{
		"clusters.cluster.x-k8s.io",
		"machines.cluster.x-k8s.io",
		"machinesets.cluster.x-k8s.io",
		"machinedeployments.cluster.x-k8s.io",
		"machinehealthchecks.cluster.x-k8s.io",
	}

	logrus.Debug("[deferred-capi] Checking CAPI CRDs availability and establishment status")
	allCRDsReady := true
	for _, crdName := range requiredCRDs {
		crd, err := w.CRD.CustomResourceDefinition().Get(crdName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Debugf("[deferred-capi] CRD %s not found, continuing to wait", crdName)
				allCRDsReady = false
				break
			}
			logrus.Debugf("[deferred-capi] Error checking for CAPI CRD %s: %v", crdName, err)
			allCRDsReady = false
			break
		}

		established := false
		for _, condition := range crd.Status.Conditions {
			if condition.Type == "Established" && condition.Status == "True" {
				established = true
				break
			}
		}

		if !established {
			logrus.Debugf("[deferred-capi] CRD %s exists but is not yet established, continuing to wait", crdName)
			allCRDsReady = false
			break
		}

		logrus.Debugf("[deferred-capi] CRD %s is available and established", crdName)
	}

	return allCRDsReady
}

func (w *Context) initializeCAPIFactory(ctx context.Context) {
	opts := &generic.FactoryOptions{
		SharedControllerFactory: w.ControllerFactory,
	}

	capi, err := capi.NewFactoryFromConfigWithOptions(w.RESTConfig, opts)
	if err != nil {
		logrus.Fatalf("Encountered unexpected error while creating capi factory: %v", err)
	}

	w.DeferredCAPIRegistration.mutex.Lock()
	defer w.DeferredCAPIRegistration.mutex.Unlock()

	w.capi = capi
	w.CAPI = w.capi.Cluster().V1beta1()

	// If the larger wrangler context has not started yet, do not start it prematurely
	w.controllerLock.Lock()
	wranglerHasStarted := w.started
	if !wranglerHasStarted {
		err = w.DeferredCAPIRegistration.invokePools(ctx, w)
		if err != nil {
			logrus.Fatalf("Encountered unexpected error while invoking deferred pools: %v", err)
		}
		w.DeferredCAPIRegistration.CAPIInitComplete = true
		logrus.Debug("[deferred-capi - initializeCAPIFactory] Not starting controller factory as primary wrangler context has not yet started")
		w.controllerLock.Unlock()
		return
	}
	w.controllerLock.Unlock()

	// If wrangler has already started, start the factory again to pick up new registrations
	if err := w.StartFactoryWithTransaction(ctx, func(ctx context.Context) error {
		err = w.DeferredCAPIRegistration.invokePools(ctx, w)
		if err != nil {
			logrus.Fatalf("Encountered unexpected error while invoking deferred pools: %v", err)
		}
		return nil
	}); err != nil {
		logrus.Fatalf("failed to invoke deferrred function pools")
	}

	logrus.Debug("[deferred-capi - initializeCAPIFactory] Starting controller factory after initial wrangler start")
	if err := w.ControllerFactory.Start(ctx, defaultControllerWorkerCount); err != nil {
		logrus.Fatalf("Encountered unexpected error while starting capi factory: %v", err)
	}

	w.DeferredCAPIRegistration.CAPIInitComplete = true
	logrus.Debug("[deferred-capi - initializeCAPIFactory] CAPI factory initialization complete")
}

type DeferredCAPIRegistration struct {
	CAPIInitComplete bool

	wg    *sync.WaitGroup
	mutex sync.Mutex

	registrationFuncs []func(ctx context.Context, clients *Context) error
	funcs             []func(clients *Context)
}

func (d *DeferredCAPIRegistration) CAPIInitialized() bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.CAPIInitComplete
}

// invokePools sequentially executes all functions pooled within the DeferredCAPIRegistration.registrationFuncs and
// DeferredCAPIRegistration.funcs slices, in that order. The caller of invokePools must first acquire
// the lock on DeferredCAPIRegistration.mutex. Once all functions from both slices have been invoked, the
// slices are reset.
func (d *DeferredCAPIRegistration) invokePools(ctx context.Context, clients *Context) error {
	logrus.Debug("[deferred-capi - invokePools] Executing deferred registration function pool")
	err := d.invokeRegistrationFuncs(ctx, clients, d.registrationFuncs)
	if err != nil {
		return err
	}
	logrus.Debug("[deferred-capi - invokePools] deferred registration functions have completed")

	logrus.Debug("[deferred-capi - invokePools] Executing deferred function pool")
	for _, f := range d.funcs {
		f(clients)
		d.wg.Done()
	}
	logrus.Debug("[deferred-capi - invokePools] deferred functions have completed")

	d.registrationFuncs = []func(ctx context.Context, clients *Context) error{}
	d.funcs = []func(clients *Context){}

	return nil
}

// DeferFunc enqueues a function to be executed once the CAPI CRDs are available by adding it to the function pool.
// Calls to DeferFunc are processed in the order they are made. Calls to DeferFunc made after the CAPI CRDs are
// available will execute immediately.
func (d *DeferredCAPIRegistration) DeferFunc(clients *Context, f func(clients *Context)) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.CAPIInitComplete {
		logrus.Debug("[deferred-capi - DeferFunc] Executing deferred function as CAPI is initialized")
		defer func() {
			logrus.Debug("[deferred-capi - DeferFunc] deferred function has completed")
		}()
		f(clients)
		return
	}

	d.wg.Add(1)
	logrus.Debug("[deferred-capi - DeferFunc] Adding function to pool")
	d.funcs = append(d.funcs, f)
}

// DeferFuncWithError creates a new go routine which invokes f once the DeferredCAPIRegistration wait group completes.
// It returns an error channel to indicate if f encountered any errors during execution.
func (d *DeferredCAPIRegistration) DeferFuncWithError(clients *Context, f func(wrangler *Context) error) chan error {
	errChan := make(chan error, 1)
	go func(errs chan error) {
		d.wg.Wait()
		logrus.Debug("[deferred-capi - DeferFuncWithError] Executing deferred function with error as CAPI is initialized")
		defer func() {
			logrus.Debug("[deferred-capi - DeferFuncWithError] deferred function with error has completed")
		}()
		err := f(clients)
		defer close(errChan)

		if err != nil {
			errChan <- err
		}
	}(errChan)
	return errChan
}

// DeferRegistration enqueues a function to be executed once the CAPI CRDs are available by adding it to the registration function pool.
// The functions passed to DeferRegistration are expected to register one or more event handlers which rely on CAPI clients.
// Functions which must be deferred, but do not register event handlers, should be passed to DeferFunc instead.
// Calls to DeferRegistration are processed in the order they are made. Calls to DeferRegistration made after the CAPI CRDs are
// available will execute immediately, and the controller factory will be immediately started.
func (d *DeferredCAPIRegistration) DeferRegistration(ctx context.Context, clients *Context, register func(ctx context.Context, clients *Context) error) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.wg.Add(1)

	if d.CAPIInitComplete {
		logrus.Debug("[deferred-capi - DeferRegistration] Executing deferred registration function as CAPI is initialized")
		defer func() {
			logrus.Debug("[deferred-capi - DeferRegistration] deferred registration function has completed")
		}()

		invoke := func() (bool, error) {
			clients.controllerLock.Lock()
			defer clients.controllerLock.Unlock()
			wranglerStarted := clients.started
			if !wranglerStarted {
				logrus.Debug("[deferred-capi - DeferRegistration] wrangler context has not yet started, will not start controller factory after registration")
				return true, d.invokeRegistrationFuncs(ctx, clients, []func(ctx context.Context, clients *Context) error{register})
			}
			return false, nil
		}

		invoked, err := invoke()
		if invoked {
			if err != nil {
				return err
			}
			return nil
		}

		return clients.StartFactoryWithTransaction(ctx, func(ctx context.Context) error {
			return d.invokeRegistrationFuncs(ctx, clients, []func(ctx context.Context, clients *Context) error{register})
		})
	}

	logrus.Debug("[deferred-capi - DeferRegistration] Adding registration function to pool")
	d.registrationFuncs = append(d.registrationFuncs, register)
	return nil
}

func (d *DeferredCAPIRegistration) invokeRegistrationFuncs(transaction context.Context, clients *Context, f []func(ctx context.Context, clients *Context) error) error {
	for _, register := range f {
		if err := register(transaction, clients); err != nil {
			return err
		}
		d.wg.Done()
	}
	return nil
}
