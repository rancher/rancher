package wrangler

import (
	"context"
	"sync/atomic"

	capi "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	wapiextv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// CAPIContext is a scoped context which wraps the larger Wrangler context.
// It includes CAPI clients and factories which are initialized after CAPI
// CRDs are detected.
type CAPIContext struct {
	*Context
	CAPI        capicontrollers.Interface
	CAPIFactory *capi.Factory
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

	logrus.Info("[deferred-capi - WaitForClient] CRDs found, initializing CAPI factory")
	opts := &generic.FactoryOptions{
		SharedControllerFactory: d.context.ControllerFactory,
	}

	capi, err := capi.NewFactoryFromConfigWithOptions(d.context.RESTConfig, opts)
	if err != nil {
		logrus.Fatalf("Encountered unexpected error while creating capi factory: %v", err)
	}

	return &CAPIContext{
		Context:     d.context,
		CAPIFactory: capi,
		CAPI:        capi.Cluster().V1beta1(),
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
