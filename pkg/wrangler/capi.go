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
	*BaseInitializer[*CAPIContext]
}

func NewCAPIInitializer() *DeferredCAPIInitializer {
	return &DeferredCAPIInitializer{
		BaseInitializer: NewBaseInitializer[*CAPIContext](),
	}
}

func (d *DeferredCAPIInitializer) OnChange(ctx context.Context, c *Context) {
	var done atomic.Bool
	c.CRD.CustomResourceDefinition().OnChange(ctx, "capi-deferred-registration", func(key string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
		if done.Load() {
			return crd, nil
		}

		if !capiCRDsReady(c.CRD.CustomResourceDefinition().Cache()) {
			return crd, nil
		}

		if !done.CompareAndSwap(false, true) {
			return crd, nil
		}

		logrus.Info("[deferred-capi - OnChange] initializing CAPI factory")
		opts := &generic.FactoryOptions{
			SharedControllerFactory: c.ControllerFactory,
		}

		capi, err := capi.NewFactoryFromConfigWithOptions(c.RESTConfig, opts)
		if err != nil {
			logrus.Fatalf("Encountered unexpected error while creating capi factory: %v", err)
		}

		d.SetClientContext(&CAPIContext{
			Context:     c,
			CAPIFactory: capi,
			CAPI:        capi.Cluster().V1beta1(),
		})

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
