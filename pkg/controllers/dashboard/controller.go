package dashboard

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard/plugin"
	"github.com/rancher/rancher/pkg/namespace"

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
	"github.com/rancher/rancher/pkg/controllers/provisioningv2"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/needacert"
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

	if features.UIPlugin.Enabled() {
		plugin.Register(ctx, namespace.UIPluginNamespace, wrangler.Catalog.UIPlugin(), wrangler.Catalog.UIPlugin().Cache())
	}

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
			capr.Register(ctx, wrangler, kubeconfigManager)
		}
	}

	if features.MCMAgent.Enabled() || features.MCM.Enabled() {
		err := mcmagent.Register(ctx, wrangler)
		if err != nil {
			return err
		}
	}

	return nil
}
