package dashboard

import (
	"context"

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
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/needacert"
)

func Register(ctx context.Context, wrangler *wrangler.Context, embedded bool, registryOverride string) error {
	helm.Register(ctx, wrangler)
	kubernetesprovider.Register(ctx,
		wrangler.Mgmt.Cluster(),
		wrangler.K8s,
		wrangler.MultiClusterManager)
	apiservice.Register(ctx, wrangler, embedded)
	needacert.Register(ctx,
		wrangler.Core.Secret(),
		wrangler.Core.Service(),
		wrangler.Admission.MutatingWebhookConfiguration(),
		wrangler.Admission.ValidatingWebhookConfiguration(),
		wrangler.CRD.CustomResourceDefinition())
	scaleavailable.Register(ctx, wrangler)
	if err := systemcharts.Register(ctx, wrangler, registryOverride); err != nil {
		return err
	}

	if err := cspadaptercharts.Register(ctx, wrangler); err != nil {
		return err
	}

	clusterconnected.Register(ctx, wrangler)

	if features.MCM.Enabled() {
		hostedcluster.Register(ctx, wrangler)
	}

	if features.Fleet.Enabled() {
		if err := fleetcharts.Register(ctx, wrangler); err != nil {
			return err
		}
	}

	if features.ProvisioningV2.Enabled() || features.MCM.Enabled() {
		clusterregistrationtoken.Register(ctx, wrangler)
	}

	if features.ProvisioningV2.Enabled() {
		kubeconfigManager := kubeconfig.New(wrangler)
		clusterindex.Register(ctx, wrangler)
		provisioningv2.Register(ctx, wrangler, kubeconfigManager)
		if features.RKE2.Enabled() {
			if err := capr.Register(ctx, wrangler, kubeconfigManager); err != nil {
				return err
			}
		}
	}

	// In the case where Rancher is embedded and running in the Harvester local cluster,
	// we need to manage the SystemUpgradeControllerReady condition for the local cluster.
	if !features.MCMAgent.Enabled() && !features.MCM.Enabled() && features.Harvester.Enabled() {
		h := rkecontrolplanecondition.Handler{
			MgmtClusterName:      "local",
			ClusterCache:         wrangler.Provisioning.Cluster().Cache(),
			DownstreamAppClient:  wrangler.Catalog.App(),
			DownstreamPlanClient: wrangler.Plan.Plan(),
		}
		rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, wrangler.RKE.RKEControlPlane(),
			"", "sync-suc-condition-harvester-local", h.SyncSystemUpgradeControllerCondition)
	}

	if features.MCMAgent.Enabled() || features.MCM.Enabled() {
		err := mcmagent.Register(ctx, wrangler)
		if err != nil {
			return err
		}
	}

	return nil
}
