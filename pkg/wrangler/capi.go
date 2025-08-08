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

	logrus.Debug("[deferred-capi] Starting to monitor CAPI CRD availability")

	for {
		allCRDsReady := w.checkCAPICRDs()
		if allCRDsReady {
			logrus.Debug("[deferred-capi] All CAPI CRDs are now available and established.")
			w.createCAPIFactoryAndStart(ctx)
			return
		}

		select {
		case <-ctx.Done():
			logrus.Error("[deferred-capi] Context cancelled while waiting for CAPI CRDs")
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

func (w *Context) createCAPIFactoryAndStart(ctx context.Context) {
	opts := &generic.FactoryOptions{
		SharedControllerFactory: w.ControllerFactory,
	}

	capi, err := capi.NewFactoryFromConfigWithOptions(w.RESTConfig, opts)
	if err != nil {
		panic(err)
	}
	w.capi = capi
	w.CAPI = w.capi.Cluster().V1beta1()

	err = w.DeferredCAPIRegistration.invokePools(ctx, w)
	if err != nil {
		// TODO: We shouldn't panic. Right?
		panic(err)
	}

	if err := w.SharedControllerFactory.Start(ctx, 50); err != nil {
		panic(err)
	}

	w.DeferredCAPIRegistration.CAPIInitialized = true
}

type DeferredCAPIRegistration struct {
	CAPIInitialized bool
	wg              *sync.WaitGroup

	lock sync.Mutex

	registrationFuncs []func(ctx context.Context, clients *Context) error
	funcs             []func(clients *Context)
}

func (d *DeferredCAPIRegistration) invokePools(ctx context.Context, clients *Context) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if err := clients.StartSharedFactoryWithTransaction(ctx, func(ctx context.Context) error {
		return d.invokeRegistrationFuncs(ctx, clients, d.registrationFuncs)
	}); err != nil {
		return err
	}

	for _, f := range d.funcs {
		f(clients)
		d.wg.Done()
	}

	d.registrationFuncs = []func(ctx context.Context, clients *Context) error{}
	d.funcs = []func(clients *Context){}

	return nil
}

func (d *DeferredCAPIRegistration) DeferFunc(clients *Context, f func(clients *Context)) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.CAPIInitialized {
		f(clients)
		return
	}
	d.wg.Add(1)
	d.funcs = append(d.funcs, f)
}

func (d *DeferredCAPIRegistration) DeferFuncWithError(clients *Context, f func(wrangler *Context) error) chan error {
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

func (d *DeferredCAPIRegistration) DeferRegistration(ctx context.Context, clients *Context, register func(ctx context.Context, clients *Context) error) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.wg.Add(1)
	if d.CAPIInitialized {
		return clients.StartSharedFactoryWithTransaction(ctx, func(ctx context.Context) error {
			if err := d.invokeRegistrationFuncs(ctx, clients, []func(ctx context.Context, clients *Context) error{register}); err != nil {
				return err
			}
			return nil
		})
	}
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
