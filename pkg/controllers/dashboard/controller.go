package dashboard

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/capr"
	"github.com/rancher/rancher/pkg/controllers/dashboard/apiservice"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterindex"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/controllers/dashboard/cspadaptercharts"
	"github.com/rancher/rancher/pkg/controllers/dashboard/fleetcharts"
	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/controllers/dashboard/hostedcluster"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	"github.com/rancher/rancher/pkg/controllers/dashboard/mcmagent"
	"github.com/rancher/rancher/pkg/controllers/dashboard/scaleavailable"
	"github.com/rancher/rancher/pkg/controllers/dashboard/systemcharts"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rkecontrolplanecondition"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/needacert"
)

func Register(ctx context.Context, clients *wrangler.Context, embedded bool, registryOverride string) error {
	helm.Register(ctx, clients)
	kubernetesprovider.Register(ctx,
		clients.Mgmt.Cluster(),
		clients.K8s,
		clients.MultiClusterManager)
	apiservice.Register(ctx, clients, embedded)
	needacert.Register(ctx,
		clients.Core.Secret(),
		clients.Core.Service(),
		clients.Admission.MutatingWebhookConfiguration(),
		clients.Admission.ValidatingWebhookConfiguration(),
		clients.CRD.CustomResourceDefinition())
	scaleavailable.Register(ctx, clients)
	if err := systemcharts.Register(ctx, clients, registryOverride); err != nil {
		return err
	}

	if err := cspadaptercharts.Register(ctx, clients); err != nil {
		return err
	}

	clusterconnected.Register(ctx, clients)

	if features.MCM.Enabled() {
		hostedcluster.Register(ctx, clients)
	}

	if features.Fleet.Enabled() {
		if err := fleetcharts.Register(ctx, clients); err != nil {
			return err
		}
	}

	if features.ProvisioningV2.Enabled() || features.MCM.Enabled() {
		clusterregistrationtoken.Register(ctx, clients)
	}

	if features.ProvisioningV2.Enabled() {
		kubeconfigManager := kubeconfig.New(clients)
		clusterindex.Register(ctx, clients)

		provisioningv2.EarlyRegister(ctx, clients, kubeconfigManager)
		if features.RKE2.Enabled() {
			if err := capr.EarlyRegister(ctx, clients); err != nil {
				return fmt.Errorf("failed to register capr controllers")
			}
		}

		// defer registration of controllers which have CAPI clients or use CAPI caches
		clients.DeferredCAPIRegistration.DeferRegistration(func(ctx context.Context, clients *wrangler.CAPIContext) error {
			provisioningv2.Register(ctx, clients, kubeconfigManager)
			if features.RKE2.Enabled() {
				if err := capr.Register(ctx, clients, kubeconfigManager); err != nil {
					return fmt.Errorf("failed to register deferred capr controllers: %w", err)
				}
			}
			return nil
		})
	}

	// In the case where Rancher is embedded and running in the Harvester local cluster,
	// we need to manage the SystemUpgradeControllerReady condition for the local cluster.
	// Note that the local cluster is treated as a Rancher-provisioned RKE2 cluster
	// rather than an imported one in this scenario.
	if !features.MCMAgent.Enabled() && !features.MCM.Enabled() && features.Harvester.Enabled() {
		rkecontrolplanecondition.Register(ctx,
			"local",
			clients.Provisioning.Cluster().Cache(),
			clients.Catalog.App(),
			clients.Plan.Plan(),
			clients.RKE.RKEControlPlane())
	}

	if features.MCMAgent.Enabled() || features.MCM.Enabled() {
		err := mcmagent.Register(ctx, clients)
		if err != nil {
			return err
		}
	}

	return nil
}
