package wrangler

import (
	"context"
	"sync/atomic"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/features"
	capi "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	wapiextv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CAPIContext is a scoped context which wraps the larger Wrangler context.
// It includes CAPI clients and factories which are initialized after CAPI
// CRDs are detected.
type CAPIContext struct {
	*Context
	CAPI        capicontrollers.Interface
	CAPIFactory *capi.Factory
	Client      client.Client
}

// DeferredCAPIInitializer implements the DeferredInitializer interface
// and monitors CRDs until all expected CAPI resources have been created.
type DeferredCAPIInitializer struct {
	context *Context
}

func NewCAPIInitializer(clients *Context) *DeferredCAPIInitializer {
	return &DeferredCAPIInitializer{
		context: clients,
	}
}

func (d *DeferredCAPIInitializer) WaitForClient(ctx context.Context) (*CAPIContext, error) {
	var done atomic.Bool
	ready := make(chan struct{})
	logrus.Info("[deferred-capi - WaitForClient] waiting for CAPI CRDs to be established...")
	d.context.CRD.CustomResourceDefinition().OnChange(ctx, "capi-deferred-registration", func(key string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
		if done.Load() {
			return crd, nil
		}

		if !capiCRDsReady(d.context.CRD.CustomResourceDefinition().Cache()) {
			return crd, nil
		}

		if !done.CompareAndSwap(false, true) {
			return crd, nil
		}
		close(ready)
		return crd, nil
	})

	select {
	case <-ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	logrus.Info("[deferred-capi - WaitForClient] waiting for CAPI catalog App to be ready with correct version...")

	if err := waitForCAPIAppVersion(ctx, d.context); err != nil {
		return nil, err
	}

	logrus.Info("[deferred-capi - WaitForClient] CAPI catalog App version is correct, initializing CAPI factory")

	opts := &generic.FactoryOptions{
		SharedControllerFactory: d.context.ControllerFactory,
	}

	capiFactory, err := capi.NewFactoryFromConfigWithOptions(d.context.RESTConfig, opts)
	if err != nil {
		logrus.Fatalf("Encountered unexpected error while creating capi factory: %v", err)
	}

	// Create controller-runtime client for CAPI operations using the wrangler Scheme.
	// This ensures the client can properly decode all CAPI and related resource types.
	//
	// Note: This client is not cached because controller-runtime's cache system is separate
	// from wrangler/lasso's SharedControllerFactory cache. While we could create a separate
	// controller-runtime cache, that would duplicate the existing wrangler cache infrastructure.
	// This client is primarily used by external.GetObjectFromContractVersionedRef() from the
	// CAPI library, which requires a controller-runtime client interface.
	//
	// The CAPI factory (created above) uses the wrangler SharedControllerFactory cache for
	// most operations, so API calls from this client are limited to the specific CAPI
	// contract reference lookups.
	c, err := client.New(d.context.RESTConfig, client.Options{
		Scheme: Scheme,
	})
	if err != nil {
		logrus.Fatalf("Encountered unexpected error while creating controller-runtime client: %v", err)
	}

	return &CAPIContext{
		Context:     d.context,
		CAPIFactory: capiFactory,
		CAPI:        capiFactory.Cluster().V1beta2(),
		Client:      c,
	}, nil
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
			if k8serr.IsNotFound(err) {
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

// waitForCAPIAppVersion waits for the catalog App (rancher-provisioning-capi or rancher-turtles)
// to be running with the correct version specified in settings.
// It also enqueues the rancher-charts ClusterRepo on version mismatches or errors to trigger a refresh.
func waitForCAPIAppVersion(ctx context.Context, wContext *Context) error {
	logrus.Info("[deferred-capi] Checking CAPI catalog App version...")

	var (
		name    string
		ns      string
		version string
	)

	if features.EmbeddedClusterAPI.Enabled() && features.Turtles.Enabled() {
		logrus.Debugf("[deferred-capi] Both embedded-cluster-api and turtles features are enabled, which is not supported. Skipping CAPI App version wait")
		return nil
	}

	if features.Turtles.Enabled() {
		name = chart.TurtlesChartName
		ns = namespace.TurtlesNamespace
		version = settings.RancherTurtlesVersion.Get()
	} else if features.EmbeddedClusterAPI.Enabled() {
		name = chart.ProvisioningCAPIChartName
		ns = namespace.ProvisioningCAPINamespace
		version = settings.RancherProvisioningCAPIVersion.Get()
	}

	if name == "" || ns == "" || version == "" {
		logrus.Debugf("[deferred-capi] Neither Turtles nor EmbeddedClusterAPI feature is enabled, skipping CAPI App version wait")
		return nil
	}

	check := func() bool {
		app, err := wContext.Catalog.App().Get(ns, name, metav1.GetOptions{})
		if err != nil {
			if k8serr.IsNotFound(err) {
				wContext.Catalog.ClusterRepo().Enqueue("rancher-charts")
				logrus.Tracef("[deferred-capi] App %s/%s not found, continuing to wait...", ns, name)
			} else {
				logrus.Warnf("[deferred-capi] Error getting App %s/%s: %v", ns, name, err)
			}
			return false
		}

		if app.Spec.Chart == nil || app.Spec.Chart.Metadata == nil {
			wContext.Catalog.ClusterRepo().Enqueue("rancher-charts")
			logrus.Tracef("[deferred-capi] App %s/%s has no chart metadata, continuing to wait...", ns, name)
			return false
		}

		currentVersion := app.Spec.Chart.Metadata.Version
		if currentVersion != version {
			wContext.Catalog.ClusterRepo().Enqueue("rancher-charts")
			logrus.Tracef("[deferred-capi] App %s/%s version mismatch: current=%s, expected=%s, continuing to wait...", ns, name, currentVersion, version)
			return false
		}

		if app.Status.Summary.State != string(catalog.StatusDeployed) {
			logrus.Tracef("[deferred-capi] App %s/%s is not yet deployed (current state: %s), continuing to wait...", ns, name, app.Status.Summary.State)
			return false
		}

		return true
	}

	// Initial check before setting up the watch
	if check() {
		return nil
	}

	ready := make(chan struct{})
	var done atomic.Bool

	wContext.Catalog.App().OnChange(ctx, "deferred-capi-app-version", func(_ string, app *v1.App) (*v1.App, error) {
		if done.Load() {
			return app, nil
		}
		if check() && done.CompareAndSwap(false, true) {
			close(ready)
		}
		return app, nil
	})

	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
