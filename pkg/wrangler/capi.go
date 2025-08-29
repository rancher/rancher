package wrangler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	capi "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io"
	wapiextv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// manageDeferredCAPIContext polls for the availability of CAPI CRDs and registers deferred controllers
// and executes deferred functions once they are available. Once CAPI CRDs are found, this function will
// not continue polling. Individual registration calls can be made once polling is complete by directly using
// the DeferredCAPIRegistration struct.
func (w *Context) manageDeferredCAPIContext(ctx context.Context) {
	logrus.Info("[deferred-capi - manageDeferredCAPIContext] Starting to monitor CAPI CRD availability")
	var done atomic.Bool
	w.CRD.CustomResourceDefinition().OnChange(ctx, "capi-deferred-registration", func(key string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
		if done.Load() {
			return crd, nil
		}

		if !capiCRDsReady(w.CRD.CustomResourceDefinition().Cache()) {
			return crd, nil
		}

		if !done.CompareAndSwap(false, true) {
			return crd, nil
		}

		logrus.Info("[deferred-capi - manageDeferredCAPIContext] initializing CAPI factory")
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
		w.CAPI = capi.Cluster().V1beta1()

		go func() {
			err = w.DeferredCAPIRegistration.run(ctx)
			if err != nil {
				logrus.Fatalf("failed to run loop during deferred capi registration: %v", err)
			}
		}()

		return crd, nil
	})
}

func capiCRDsReady(crdCache wapiextv1.CustomResourceDefinitionCache) bool {
	requiredCRDs := []string{
		"clusters.cluster.x-k8s.io",
		"machines.cluster.x-k8s.io",
		"machinesets.cluster.x-k8s.io",
		"machinedeployments.cluster.x-k8s.io",
		"machinehealthchecks.cluster.x-k8s.io",
	}

	logrus.Tracef("[deferred-capi] Checking CAPI CRDs availability and establishment status")
	allCRDsReady := true
	for _, crdName := range requiredCRDs {
		crd, err := crdCache.Get(crdName)
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Tracef("[deferred-capi] CRD %s not found, continuing to wait", crdName)
				allCRDsReady = false
				break
			}
			logrus.Errorf("[deferred-capi] Error checking for CAPI CRD %s: %v", crdName, err)
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
			logrus.Tracef("[deferred-capi] CRD %s exists but is not yet established, continuing to wait", crdName)
			allCRDsReady = false
			break
		}

		logrus.Tracef("[deferred-capi] CRD %s is available and established", crdName)
	}

	return allCRDsReady
}

type DeferredCAPIRegistration struct {
	mutex sync.Mutex

	clients           *Context
	registrationFuncs chan func(ctx context.Context, clients *Context) error
	funcs             chan func(clients *Context)
}

func newDeferredCAPIRegistration(clients *Context) *DeferredCAPIRegistration {
	return &DeferredCAPIRegistration{
		clients:           clients,
		registrationFuncs: make(chan func(ctx context.Context, clients *Context) error, 100),
		funcs:             make(chan func(clients *Context), 100),
	}
}

func (d *DeferredCAPIRegistration) run(ctx context.Context) error {
	for {
		select {
		case f := <-d.registrationFuncs:
			if err := d.clients.StartFactoryWithTransaction(ctx, func(ctx context.Context) error {
				if err := f(ctx, d.clients); err != nil {
					return fmt.Errorf("failed to invoke reg func: %w", err)
				}
				return nil
			}); err != nil {
				return err
			}
		case f := <-d.funcs:
			f(d.clients)
		case <-ctx.Done():
			return nil
		}
	}
}

// DeferFunc enqueues a function to be executed once the CAPI CRDs are available by adding it to the function pool.
// Calls to DeferFunc are processed in the order they are made. Calls to DeferFunc made after the CAPI CRDs are
// available will execute immediately.
func (d *DeferredCAPIRegistration) DeferFunc(f func(clients *Context)) {
	logrus.Debug("[deferred-capi - DeferFunc] Adding function to pool")
	d.funcs <- f
}

// DeferFuncWithError creates a new go routine which invokes f once the DeferredCAPIRegistration wait group completes.
// It returns an error channel to indicate if f encountered any errors during execution.
func (d *DeferredCAPIRegistration) DeferFuncWithError(f func(wrangler *Context) error) chan error {
	errChan := make(chan error, 1)
	d.funcs <- func(clients *Context) {
		defer close(errChan)

		if err := f(clients); err != nil {
			errChan <- err
		}
	}
	return errChan
}

// DeferRegistration enqueues a function to be executed once the CAPI CRDs are available by adding it to the registration function pool.
// The functions passed to DeferRegistration are expected to register one or more event handlers which rely on CAPI clients.
// Functions which must be deferred, but do not register event handlers, should be passed to DeferFunc instead.
// Calls to DeferRegistration are processed in the order they are made. Calls to DeferRegistration made after the CAPI CRDs are
// available will execute immediately, and the controller factory will be immediately started.
func (d *DeferredCAPIRegistration) DeferRegistration(register func(ctx context.Context, clients *Context) error) {
	logrus.Debug("[deferred-capi - DeferRegistration] Adding registration function to pool")
	d.registrationFuncs <- register
}
